package httpapi

import (
	"net/http"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// SAMLClient is the SAML 2.0 service provider (ADR-0017).
type SAMLClient interface {
	Enabled() bool
	Metadata() ([]byte, error)
	Start(relayState string) (redirect, requestID string, err error)
	Profile(r *http.Request, requestIDs []string) (application.ExternalProfile, error)
}

// samlCookie carries the AuthnRequest id, so the response must answer this browser's request.
const samlCookie = "kyber_saml"

func (h *Handler) saml() SAMLClient {
	if h.social.SAML == nil || !h.social.SAML.Enabled() {
		return nil
	}
	return h.social.SAML
}

func (h *Handler) registerSAML(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/saml/metadata", h.samlMetadata)
	mux.HandleFunc("GET /api/v1/auth/saml/start", h.samlStart)
	mux.HandleFunc("POST /api/v1/auth/saml/acs", h.samlACS)
}

func (h *Handler) samlMetadata(w http.ResponseWriter, r *http.Request) {
	sp := h.saml()
	if sp == nil {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	md, err := sp.Metadata()
	if err != nil {
		httpx.Error(w, r, h.log, err, codeFor)
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write(md)
}

// samlCookieFor: the response arrives as a cross-site POST from the identity provider, which only
// carries SameSite=None cookies (and those must be Secure).
func (h *Handler) samlCookieFor(value string, maxAge int) *http.Cookie {
	c := &http.Cookie{Name: samlCookie, Value: value, Path: "/api/v1/auth/saml/", MaxAge: maxAge, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteLaxMode}
	if h.cookieSecure {
		c.SameSite = http.SameSiteNoneMode
	}
	return c
}

func (h *Handler) samlStart(w http.ResponseWriter, r *http.Request) {
	sp := h.saml()
	if sp == nil {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	to, id, err := sp.Start("")
	if err != nil {
		h.failed(w, r, "saml", err)
		return
	}
	http.SetCookie(w, h.samlCookieFor(id, 600))
	http.Redirect(w, r, to, http.StatusFound)
}

func (h *Handler) samlACS(w http.ResponseWriter, r *http.Request) {
	sp := h.saml()
	if sp == nil {
		httpx.Error(w, r, h.log, errProviderDisabled, codeFor)
		return
	}
	http.SetCookie(w, h.samlCookieFor("", -1))
	var ids []string
	if c, err := r.Cookie(samlCookie); err == nil && c.Value != "" {
		ids = []string{c.Value}
	}
	if len(ids) == 0 { // no request of ours in this browser: identity-provider-initiated or forged
		h.failed(w, r, "saml", application.ErrInvalidLogin)
		return
	}
	p, err := sp.Profile(r, ids)
	if err != nil {
		h.failed(w, r, "saml", err)
		return
	}
	h.finishSocial(w, r, "saml", p)
}
