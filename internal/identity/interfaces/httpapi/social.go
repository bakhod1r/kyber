package httpapi

import (
	"context"

	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/bakhod1r/errorx"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// Social configures sign-in with external providers; nil members are disabled.
type Social struct {
	Google      OAuthClient
	OIDC        OAuthClient // the installation's generic OpenID Connect provider (SSO)
	OIDCName    string      // its button label, e.g. "Okta"
	GitHub      OAuthClient
	Telegram    TelegramVerifier
	TelegramBot string // bot username for the login widget and the code bot
	TelegramOTP bool   // the bot sends one-time login codes (needs the bot token)
}

// OAuthClient is a redirect (authorization code) sign-in provider.
type OAuthClient interface {
	Enabled() bool
	AuthURL(ctx context.Context, f application.OIDCFlow) (string, error)
	Exchange(ctx context.Context, code string, f application.OIDCFlow) (application.ExternalProfile, error)
}

type TelegramVerifier interface {
	Enabled() bool
	Verify(q url.Values) (application.ExternalProfile, error)
}

const flowCookie = "kyber_oauth"

var errProviderDisabled = errors.New("this sign-in method is not enabled")

// WithSocial enables the social sign-in routes registered by RegisterPublic.
func (h *Handler) WithSocial(s Social) *Handler { h.social = s; return h }

func (h *Handler) registerSocial(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/providers", h.providers)
	for _, name := range []string{"google", "oidc", "github"} {
		mux.HandleFunc("GET /api/v1/auth/"+name+"/start", h.oauthStart(name))
		mux.HandleFunc("GET /api/v1/auth/"+name+"/callback", h.oauthCallback(name))
	}
	mux.HandleFunc("POST /api/v1/auth/telegram", h.telegram)
	mux.HandleFunc("POST /api/v1/auth/telegram/otp", h.otpStart)
	mux.HandleFunc("POST /api/v1/auth/telegram/otp/verify", h.otpVerify)
}

// oauth returns the enabled redirect provider by its route name, or nil.
func (h *Handler) oauth(name string) OAuthClient {
	c := map[string]OAuthClient{"google": h.social.Google, "oidc": h.social.OIDC, "github": h.social.GitHub}[name]
	if c == nil || !c.Enabled() {
		return nil
	}
	return c
}

func (h *Handler) telegramOn() bool { return h.social.Telegram != nil && h.social.Telegram.Enabled() }

func (h *Handler) providers(w http.ResponseWriter, _ *http.Request) {
	out := struct {
		Google      bool    `json:"google"`
		OIDC        *string `json:"oidc"`
		GitHub      bool    `json:"github"`
		TelegramBot *string `json:"telegram_bot"`
		TelegramOTP bool    `json:"telegram_otp"`
	}{Google: h.oauth("google") != nil, GitHub: h.oauth("github") != nil, TelegramOTP: h.social.TelegramOTP}
	if h.oauth("oidc") != nil {
		name := h.social.OIDCName
		if name == "" {
			name = "SSO"
		}
		out.OIDC = &name
	}
	if h.telegramOn() {
		out.TelegramBot = &h.social.TelegramBot
	}
	httpx.JSON(w, http.StatusOK, out)
}

// failed sends the browser back to the login page with a reason it can show.
func (h *Handler) failed(w http.ResponseWriter, r *http.Request, provider string, err error) {
	if !errors.Is(err, application.ErrInvalidLogin) && !errors.Is(err, domain.ErrEmailTaken) {
		h.log.ErrorContext(r.Context(), "social sign-in failed", "provider", provider, "err", err)
	}
	reason := "failed"
	if errors.Is(err, domain.ErrEmailTaken) {
		reason = "email_taken"
	}
	http.Redirect(w, r, "/login?error="+provider+"_"+reason, http.StatusFound)
}

// oauthStart begins a redirect sign-in (Google, the SSO provider or GitHub).
func (h *Handler) oauthStart(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := h.oauth(name)
		if c == nil {
			httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
			return
		}
		f := application.NewOIDCFlow()
		u, err := c.AuthURL(r.Context(), f)
		if err != nil {
			h.failed(w, r, name, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: flowCookie, Value: f.State + "." + f.Nonce + "." + f.Verifier,
			Path: "/api/v1/auth/" + name + "/", MaxAge: 600, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, u, http.StatusFound)
	}
}

func (h *Handler) oauthCallback(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := h.oauth(name)
		if c == nil {
			httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: flowCookie, Value: "", Path: "/api/v1/auth/" + name + "/", MaxAge: -1, HttpOnly: true, Secure: h.cookieSecure})
		ck, err := r.Cookie(flowCookie)
		parts := []string{}
		if err == nil {
			parts = strings.Split(ck.Value, ".")
		}
		// The state must match the cookie set by /start in this browser (login CSRF).
		if len(parts) != 3 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(r.URL.Query().Get("state"))) != 1 {
			h.failed(w, r, name, application.ErrInvalidLogin)
			return
		}
		p, err := c.Exchange(r.Context(), r.URL.Query().Get("code"), application.OIDCFlow{State: parts[0], Nonce: parts[1], Verifier: parts[2]})
		if err != nil {
			h.failed(w, r, name, err)
			return
		}
		h.finishSocial(w, r, name, p)
	}
}

func (h *Handler) finishSocial(w http.ResponseWriter, r *http.Request, provider string, p application.ExternalProfile) {
	token, _, err := h.svc.LoginExternal(r.Context(), p)
	if err != nil {
		h.failed(w, r, provider, err)
		return
	}
	h.setSession(w, token)
	http.Redirect(w, r, "/", http.StatusFound)
}

// telegram receives the Login Widget's payload as JSON (so the CSRF guard of JSON applies)
// and answers like /auth/login.
func (h *Handler) telegram(w http.ResponseWriter, r *http.Request) {
	if !h.telegramOn() {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	if !isJSON(r) {
		httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
		return
	}
	var in map[string]any
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	q := url.Values{}
	for k, v := range in {
		switch x := v.(type) {
		case string:
			q.Set(k, x)
		case float64:
			b, _ := json.Marshal(x) // integers keep their exact digits
			q.Set(k, string(b))
		}
	}
	p, err := h.social.Telegram.Verify(q)
	if err != nil {
		httpx.Error(w, r, h.log, application.ErrInvalidCredentials, codeFor)
		return
	}
	token, _, err := h.svc.LoginExternal(r.Context(), p)
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	h.setSession(w, token)
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: h.cookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(application.SessionTTL),
	})
}

// otpStart begins "log in with a Telegram code": the browser opens the deep link, presses
// Start in the bot and receives a code there.
func (h *Handler) otpStart(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	if !h.social.TelegramOTP {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	c, err := h.svc.StartTelegramOTP(r.Context(), clientIP(r))
	var tooMany *application.TooManyAttemptsError
	if errors.As(err, &tooMany) {
		httpx.Problem(w, r, errorx.New(httpx.CodeTooManyAttempts, err.Error()).WithDetails(err.Error()).WithRetryAfter(tooMany.RetryAfter))
		return
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id": c.ID, "link": "https://t.me/" + url.PathEscape(h.social.TelegramBot) + "?start=" + c.Nonce, "expires_at": c.ExpiresAt,
	})
}

func (h *Handler) otpVerify(w http.ResponseWriter, r *http.Request) {
	if !requireJSON(w, r) {
		return
	}
	if !h.social.TelegramOTP {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	var in struct{ ID, Code string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	token, _, err := h.svc.VerifyTelegramOTP(r.Context(), in.ID, strings.TrimSpace(in.Code))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	h.setSession(w, token)
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
}

// requireJSON rejects cross-site form posts on public sign-in endpoints (login CSRF):
// browsers cannot send application/json cross-site without a CORS preflight.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	if isJSON(r) {
		return true
	}
	msg := "sign-in requests require Content-Type: application/json"
	httpx.Problem(w, r, errorx.New(httpx.CodeCSRF, msg).WithDetails(msg))
	return false
}
