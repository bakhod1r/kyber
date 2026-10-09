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

	"github.com/bakhod1r/kyber/internal/identity/adapter/google"
	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// Social configures sign-in with Google and Telegram; nil members are disabled.
type Social struct {
	Google      GoogleClient
	Telegram    TelegramVerifier
	TelegramBot string // bot username for the login widget and the code bot
	TelegramOTP bool   // the bot sends one-time login codes (needs the bot token)
}

type GoogleClient interface {
	Enabled() bool
	AuthURL(ctx context.Context, f google.Flow) (string, error)
	Exchange(ctx context.Context, code string, f google.Flow) (app.ExternalProfile, error)
}

type TelegramVerifier interface {
	Enabled() bool
	Verify(q url.Values) (app.ExternalProfile, error)
}

const flowCookie = "kyber_oauth"

var errProviderDisabled = errors.New("this sign-in method is not enabled")

// WithSocial enables the social sign-in routes registered by RegisterPublic.
func (h *Handler) WithSocial(s Social) *Handler { h.social = s; return h }

func (h *Handler) registerSocial(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/providers", h.providers)
	mux.HandleFunc("GET /api/v1/auth/google/start", h.googleStart)
	mux.HandleFunc("GET /api/v1/auth/google/callback", h.googleCallback)
	mux.HandleFunc("POST /api/v1/auth/telegram", h.telegram)
	mux.HandleFunc("POST /api/v1/auth/telegram/otp", h.otpStart)
	mux.HandleFunc("POST /api/v1/auth/telegram/otp/verify", h.otpVerify)
}

func (h *Handler) googleOn() bool   { return h.social.Google != nil && h.social.Google.Enabled() }
func (h *Handler) telegramOn() bool { return h.social.Telegram != nil && h.social.Telegram.Enabled() }

func (h *Handler) providers(w http.ResponseWriter, _ *http.Request) {
	out := struct {
		Google      bool    `json:"google"`
		TelegramBot *string `json:"telegram_bot"`
		TelegramOTP bool    `json:"telegram_otp"`
	}{Google: h.googleOn(), TelegramOTP: h.social.TelegramOTP}
	if h.telegramOn() {
		out.TelegramBot = &h.social.TelegramBot
	}
	httpx.JSON(w, http.StatusOK, out)
}

// failed sends the browser back to the login page with a reason it can show.
func (h *Handler) failed(w http.ResponseWriter, r *http.Request, provider string, err error) {
	if !errors.Is(err, google.ErrInvalidLogin) && !errors.Is(err, domain.ErrEmailTaken) {
		h.log.ErrorContext(r.Context(), "social sign-in failed", "provider", provider, "err", err)
	}
	reason := "failed"
	if errors.Is(err, domain.ErrEmailTaken) {
		reason = "email_taken"
	}
	http.Redirect(w, r, "/login?error="+provider+"_"+reason, http.StatusFound)
}

func (h *Handler) googleStart(w http.ResponseWriter, r *http.Request) {
	if !h.googleOn() {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	f := google.NewFlow()
	u, err := h.social.Google.AuthURL(r.Context(), f)
	if err != nil {
		h.failed(w, r, "google", err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: flowCookie, Value: f.State + "." + f.Nonce + "." + f.Verifier,
		Path: "/api/v1/auth/google/", MaxAge: 600, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, u, http.StatusFound)
}

func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.googleOn() {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: flowCookie, Value: "", Path: "/api/v1/auth/google/", MaxAge: -1, HttpOnly: true, Secure: h.cookieSecure})
	c, err := r.Cookie(flowCookie)
	parts := []string{}
	if err == nil {
		parts = strings.Split(c.Value, ".")
	}
	// The state must match the cookie set by /start in this browser (login CSRF).
	if len(parts) != 3 || subtle.ConstantTimeCompare([]byte(parts[0]), []byte(r.URL.Query().Get("state"))) != 1 {
		h.failed(w, r, "google", google.ErrInvalidLogin)
		return
	}
	p, err := h.social.Google.Exchange(r.Context(), r.URL.Query().Get("code"), google.Flow{State: parts[0], Nonce: parts[1], Verifier: parts[2]})
	if err != nil {
		h.failed(w, r, "google", err)
		return
	}
	h.finishSocial(w, r, "google", p)
}

func (h *Handler) finishSocial(w http.ResponseWriter, r *http.Request, provider string, p app.ExternalProfile) {
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
		httpx.Error(w, r, h.log, app.ErrInvalidCredentials, codeFor)
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
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(app.SessionTTL),
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
	var tooMany *app.TooManyAttemptsError
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
