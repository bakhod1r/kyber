// Package httpapi exposes the Identity context over REST and provides the auth middleware.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/bakhod1r/errorx"

	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

const CookieName = "kyber_session"

type Handler struct {
	svc          *app.Service
	log          *slog.Logger
	cookieSecure bool
	social       Social
}

func New(svc *app.Service, log *slog.Logger, cookieSecure bool) *Handler {
	return &Handler{svc: svc, log: log, cookieSecure: cookieSecure}
}

// RegisterPublic mounts routes reachable without a session.
func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/signup", h.signup)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	h.registerSocial(mux)
}

// RegisterProtected mounts routes that require RequireAuth.
func (h *Handler) RegisterProtected(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("GET /api/v1/me", h.me)
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func toDTO(u *domain.User) userDTO {
	return userDTO{ID: string(u.ID()), Email: u.Email().String(), Name: u.Name()}
}

func codeFor(err error) (string, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrEmptyName):
		return httpx.CodeValidation, true
	case errors.Is(err, domain.ErrEmailTaken):
		return httpx.CodeEmailTaken, true
	case errors.Is(err, app.ErrInvalidCredentials):
		return httpx.CodeInvalidCredentials, true
	case errors.Is(err, errProviderDisabled):
		return httpx.CodeProviderDisabled, true
	case errors.Is(err, app.ErrUnauthenticated):
		return httpx.CodeAuthRequired, true
	}
	return "", false
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Name, Password string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	u, err := h.svc.Signup(r.Context(), app.Signup{Email: in.Email, Name: in.Name, Password: in.Password})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(u))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	token, err := h.svc.Login(r.Context(), in.Email, in.Password, clientIP(r))
	var tooMany *app.TooManyAttemptsError
	if errors.As(err, &tooMany) {
		httpx.Problem(w, r, errorx.New(httpx.CodeTooManyAttempts, err.Error()).
			WithDetails(err.Error()).WithRetryAfter(tooMany.RetryAfter))
		return
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	h.setSession(w, token)
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token, _ := tokenFrom(r)
	if err := h.svc.Logout(r.Context(), token); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.cookieSecure})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, toDTO(UserFrom(r.Context())))
}

type ctxKey struct{}

// UserFrom returns the authenticated user set by RequireAuth.
func UserFrom(ctx context.Context) *domain.User {
	u, _ := ctx.Value(ctxKey{}).(*domain.User)
	return u
}

// RequireAuth rejects requests without a valid session cookie or Bearer token.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, fromCookie := tokenFrom(r)
		if fromCookie && !safeMethod(r.Method) && !isJSON(r) {
			// CSRF guard: browsers cannot send cross-site JSON without a CORS preflight.
			msg := "cookie-authenticated writes require Content-Type: application/json"
			httpx.Problem(w, r, errorx.New(httpx.CodeCSRF, msg).WithDetails(msg))
			return
		}
		if token == "" {
			httpx.Error(w, r, h.log, app.ErrUnauthenticated, codeFor)
			return
		}
		u, err := h.svc.Authenticate(r.Context(), token)
		if err != nil {
			httpx.Error(w, r, h.log, err, codeFor)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, u)
		next.ServeHTTP(w, r.WithContext(auth.WithActor(ctx, string(u.ID()))))
	})
}

// tokenFrom prefers the Bearer header; fromCookie reports a cookie-sourced token.
func tokenFrom(r *http.Request) (token string, fromCookie bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer "), false
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value, true
	}
	return "", false
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func isJSON(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/json"
}

// clientIP is the TCP peer address; deployments behind a proxy should terminate
// throttling there or configure trusted forwarding (not yet supported).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
