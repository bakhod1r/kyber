package saml_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	crewjam "github.com/crewjam/saml"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	kyber "github.com/bakhod1r/kyber/internal/identity/infrastructure/saml"
)

func keyPair(t *testing.T, cn string) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

type fixed struct{ s *crewjam.Session }

func (f fixed) GetSession(http.ResponseWriter, *http.Request, *crewjam.IdpAuthnRequest) *crewjam.Session {
	return f.s
}

type oneSP struct{ md *crewjam.EntityDescriptor }

func (o oneSP) GetServiceProvider(*http.Request, string) (*crewjam.EntityDescriptor, error) {
	return o.md, nil
}

// idp is a real (crewjam) identity provider signing with its own key.
func idp(t *testing.T, session *crewjam.Session) *crewjam.IdentityProvider {
	key, cert := keyPair(t, "idp")
	meta, _ := url.Parse("https://idp.example/metadata")
	sso, _ := url.Parse("https://idp.example/sso")
	return &crewjam.IdentityProvider{Key: key, Certificate: cert, MetadataURL: *meta, SSOURL: *sso, SessionProvider: fixed{session}}
}

// signIn runs SP-initiated sign-in against the identity provider and returns the POST to the ACS.
func signIn(t *testing.T, sp *kyber.SP, provider *crewjam.IdentityProvider) (*http.Request, string) {
	t.Helper()
	redirect, requestID, err := sp.Start("relay")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(redirect, "https://idp.example/sso?SAMLRequest=") {
		t.Fatalf("redirect = %s", redirect)
	}
	spMeta, _ := sp.Metadata()
	var md crewjam.EntityDescriptor
	if err := xml.Unmarshal(spMeta, &md); err != nil {
		t.Fatal(err)
	}
	provider.ServiceProviderProvider = oneSP{&md}
	req, err := crewjam.NewIdpAuthnRequest(provider, httptest.NewRequest("GET", redirect, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (crewjam.DefaultAssertionMaker{}).MakeAssertion(req, provider.SessionProvider.GetSession(nil, nil, req)); err != nil {
		t.Fatal(err)
	}
	form, err := req.PostBinding()
	if err != nil {
		t.Fatal(err)
	}
	post := httptest.NewRequest("POST", form.URL, strings.NewReader(url.Values{"SAMLResponse": {form.SAMLResponse}, "RelayState": {form.RelayState}}.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return post, requestID
}

func newSP(t *testing.T, provider *crewjam.IdentityProvider) *kyber.SP {
	t.Helper()
	idpMeta, _ := xml.Marshal(provider.Metadata())
	key, cert := keyPair(t, "kyber")
	sp, err := kyber.New(kyber.Config{BaseURL: "https://kyber.example", IDPMetadata: idpMeta, Key: key, Cert: cert})
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestSignIn(t *testing.T) {
	provider := idp(t, &crewjam.Session{ID: "s", NameID: "ann@corp.example", UserEmail: "ann@corp.example", UserCommonName: "Ann Lee",
		CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour)})
	sp := newSP(t, provider)
	meta, _ := sp.Metadata()
	if !strings.Contains(string(meta), "https://kyber.example/api/v1/auth/saml/acs") {
		t.Fatalf("metadata lacks the ACS: %s", meta)
	}
	post, requestID := signIn(t, sp, provider)
	p, err := sp.Profile(post, []string{requestID})
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != domain.ProviderSAML || p.Email != "ann@corp.example" || !p.EmailVerified || p.Subject != "ann@corp.example" || p.Name != "Ann Lee" {
		t.Fatalf("profile = %+v", p)
	}

	// The response must answer this browser's request, and must not be accepted from another IdP.
	post, _ = signIn(t, sp, provider)
	if _, err := sp.Profile(post, []string{"id-someone-else"}); !errors.Is(err, application.ErrInvalidLogin) {
		t.Fatalf("unsolicited response: %v", err)
	}
	impostor := idp(t, &crewjam.Session{ID: "s", NameID: "ann@corp.example", CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour)})
	post, requestID = signIn(t, sp, impostor) // signed with a key the SP does not trust
	if _, err := sp.Profile(post, []string{requestID}); !errors.Is(err, application.ErrInvalidLogin) {
		t.Fatalf("foreign signature accepted: %v", err)
	}
}

func TestNoEmail(t *testing.T) {
	provider := idp(t, &crewjam.Session{ID: "s", NameID: "opaque-id", CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour)})
	sp := newSP(t, provider)
	post, requestID := signIn(t, sp, provider)
	if _, err := sp.Profile(post, []string{requestID}); !errors.Is(err, kyber.ErrNoEmail) {
		t.Fatalf("no email: %v", err)
	}
	if _, err := kyber.New(kyber.Config{IDPMetadata: []byte("<not xml")}); err == nil {
		t.Fatal("bad metadata accepted")
	}
}
