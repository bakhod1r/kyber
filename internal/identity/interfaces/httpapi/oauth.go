package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// OAuth 2.1 for MCP clients (ADR-0016). Discovery and the client-facing endpoints speak plain
// OAuth JSON ({"error": ...}); the consent endpoints the web app calls speak problem+json.

// mcpPath is the protected resource OAuth clients ask access for.
const mcpPath = "/api/v1/mcp"

// WithPublicURL sets the base URL published in the OAuth metadata (e.g. https://kyber.example.com).
func (h *Handler) WithPublicURL(u string) *Handler { h.publicURL = strings.TrimRight(u, "/"); return h }

func (h *Handler) registerOAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/api/v1/mcp", h.resourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.serverMetadata)
	mux.HandleFunc("POST /api/v1/oauth/register", h.registerClient)
	mux.HandleFunc("POST /api/v1/oauth/token", h.token)
}

// base is the server's external URL: configured, or the request's scheme and host.
func (h *Handler) base(r *http.Request) string {
	if h.publicURL != "" {
		return h.publicURL
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// challenge tells an MCP client where to start OAuth (RFC 9728 §5.1).
func (h *Handler) challenge(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == mcpPath {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+h.base(r)+`/.well-known/oauth-protected-resource"`)
	}
}

func (h *Handler) resourceMetadata(w http.ResponseWriter, r *http.Request) {
	b := h.base(r)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"resource": b + mcpPath, "authorization_servers": []string{b}, "bearer_methods_supported": []string{"header"},
	})
}

func (h *Handler) serverMetadata(w http.ResponseWriter, r *http.Request) {
	b := h.base(r)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"issuer":                                b,
		"authorization_endpoint":                b + "/oauth/authorize",
		"token_endpoint":                        b + "/api/v1/oauth/token",
		"registration_endpoint":                 b + "/api/v1/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, status, map[string]string{"error": code, "error_description": desc})
}

// registerClient is RFC 7591 dynamic client registration for public clients.
func (h *Handler) registerClient(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string   `json:"client_name"`
		Redirects []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the body must be JSON client metadata")
		return
	}
	c, err := h.svc.RegisterClient(r.Context(), clientIP(r), in.Name, in.Redirects)
	var tooMany *application.TooManyAttemptsError
	switch {
	case errors.As(err, &tooMany):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(tooMany.RetryAfter.Seconds()))))
		oauthError(w, http.StatusTooManyRequests, "too_many_requests", "too many client registrations from this address; retry later")
	case errors.Is(err, domain.ErrInvalidOAuthClient) && strings.Contains(err.Error(), "redirect_uri"):
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
	case errors.Is(err, domain.ErrInvalidOAuthClient), errors.Is(err, application.ErrOAuthDisabled):
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", err.Error())
	case err != nil:
		httpx.Error(w, r, h.log, err, codeFor)
	default:
		httpx.JSON(w, http.StatusCreated, map[string]any{
			"client_id": string(c.ID), "client_name": c.Name, "redirect_uris": c.RedirectURIs, "client_id_issued_at": c.CreatedAt.Unix(),
			"token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code"}, "response_types": []string{"code"},
		})
	}
}

// token exchanges an authorization code (form-encoded, RFC 6749 §4.1.3).
func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}
	f := r.PostForm
	token, ttl, err := h.svc.ExchangeCode(r.Context(), application.TokenRequest{
		Code: f.Get("code"), ClientID: f.Get("client_id"), RedirectURI: f.Get("redirect_uri"), Verifier: f.Get("code_verifier"),
	})
	if errors.Is(err, application.ErrInvalidGrant) || errors.Is(err, application.ErrOAuthDisabled) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code is invalid, expired or does not match")
		return
	}
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": int(ttl.Seconds())})
}

// oauthClient shows the consent screen which client is asking.
func (h *Handler) oauthClient(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.OAuthClient(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"client_id": string(c.ID), "client_name": c.Name, "redirect_uris": c.RedirectURIs})
}

// authorize records the signed-in user's approval and returns where to send the browser. Only a
// browser session may approve: a token must not be able to grant itself more tokens.
func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) {
	if !h.sessionOnly(w, r) {
		return
	}
	if !isJSON(r) { // no simple (form or text) cross-site posts can approve a client
		httpx.Error(w, r, h.log, httpx.ErrBadJSON, codeFor)
		return
	}
	var in struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		Challenge   string `json:"code_challenge"`
		Method      string `json:"code_challenge_method"`
		State       string `json:"state"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	code, err := h.svc.Authorize(r.Context(), domain.UserID(auth.Actor(r.Context())),
		application.AuthorizeRequest{ClientID: in.ClientID, RedirectURI: in.RedirectURI, Challenge: in.Challenge, Method: in.Method})
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	to, _ := url.Parse(in.RedirectURI) // validated by Authorize
	q := to.Query()
	q.Set("code", code)
	if in.State != "" {
		q.Set("state", in.State)
	}
	q.Set("iss", h.base(r))
	to.RawQuery = q.Encode()
	httpx.JSON(w, http.StatusOK, map[string]string{"redirect_to": to.String()})
}
