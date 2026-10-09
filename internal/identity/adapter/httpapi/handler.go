// Package httpapi exposes the Identity context over REST and provides the auth middleware.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

const CookieName = "kyber_session"

type Handler struct {
	svc          *app.Service
	log          *slog.Logger
	cookieSecure bool
}

func New(svc *app.Service, log *slog.Logger, cookieSecure bool) *Handler {
	return &Handler{svc: svc, log: log, cookieSecure: cookieSecure}
}

// RegisterPublic mounts routes reachable without a session.
func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/signup", h.signup)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
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

func statusFor(err error) (int, bool) {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail), errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrEmptyName):
		return http.StatusUnprocessableEntity, true
	case errors.Is(err, domain.ErrEmailTaken):
		return http.StatusConflict, true
	case errors.Is(err, app.ErrInvalidCredentials), errors.Is(err, app.ErrUnauthenticated):
		return http.StatusUnauthorized, true
	}
	return 0, false
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Name, Password string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	u, err := h.svc.Signup(r.Context(), app.Signup{Email: in.Email, Name: in.Name, Password: in.Password})
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(u))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	token, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: h.cookieSecure,
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(app.SessionTTL),
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), tokenFrom(r)); err != nil {
		httpx.Error(w, r, h.log, err, statusFor)
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
		token := tokenFrom(r)
		if token == "" {
			httpx.Error(w, r, h.log, app.ErrUnauthenticated, statusFor)
			return
		}
		u, err := h.svc.Authenticate(r.Context(), token)
		if err != nil {
			httpx.Error(w, r, h.log, err, statusFor)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func tokenFrom(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if c, err := r.Cookie(CookieName); err == nil {
		return c.Value
	}
	return ""
}
