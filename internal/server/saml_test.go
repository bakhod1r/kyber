package server_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	crewjam "github.com/crewjam/saml"

	samlsp "github.com/bakhod1r/kyber/internal/identity/infrastructure/saml"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/server"
)

func samlKeys(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "t"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

type samlSession struct{}

func (samlSession) GetSession(http.ResponseWriter, *http.Request, *crewjam.IdpAuthnRequest) *crewjam.Session {
	return &crewjam.Session{ID: "s", NameID: "sam@corp.example", UserEmail: "sam@corp.example", UserCommonName: "Sam Corp",
		CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour)}
}

type samlSPs struct{ md *crewjam.EntityDescriptor }

func (s samlSPs) GetServiceProvider(*http.Request, string) (*crewjam.EntityDescriptor, error) {
	return s.md, nil
}

// ADR-0017: SP-initiated SAML sign-in through the HTTP API.
func TestSAMLSignIn(t *testing.T) {
	key, cert := samlKeys(t)
	meta, _ := url.Parse("https://idp.example/metadata")
	sso, _ := url.Parse("https://idp.example/sso")
	idp := &crewjam.IdentityProvider{Key: key, Certificate: cert, MetadataURL: *meta, SSOURL: *sso, SessionProvider: samlSession{}}
	idpMeta, _ := xml.Marshal(idp.Metadata())
	spKey, spCert := samlKeys(t)
	// The ACS URL in the SP's metadata must be where the test server listens: start it first.
	handler := http.NewServeMux()
	srv := httptest.NewServer(handler)
	defer srv.Close()
	sp, err := samlsp.New(samlsp.Config{BaseURL: srv.URL, IDPMetadata: idpMeta, Key: spKey, Cert: spCert})
	if err != nil {
		t.Fatal(err)
	}
	handler.Handle("/", server.NewInMemory(slog.New(slog.DiscardHandler), server.WithContext(t.Context()), server.WithSocial(identityhttp.Social{SAML: sp, SAMLName: "Corp"})))
	c := &client{t: t, srv: srv}

	code, prov := c.do("GET", "/api/v1/auth/providers", nil)
	expect(t, code, 200, prov)
	if prov["saml"] != "Corp" {
		t.Fatalf("providers = %v", prov)
	}
	res := c.raw("GET", "/api/v1/auth/saml/metadata", nil, nil)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Type"), "samlmetadata") {
		t.Fatalf("metadata = %d", res.StatusCode)
	}

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	startReq, _ := http.NewRequest("GET", srv.URL+"/api/v1/auth/saml/start", nil)
	start, err := noFollow.Do(startReq)
	if err != nil {
		t.Fatal(err)
	}
	spec.check(t, startReq, start)
	start.Body.Close()
	var reqCookie *http.Cookie
	for _, ck := range start.Cookies() {
		if ck.Name == "kyber_saml" {
			reqCookie = ck
		}
	}
	if start.StatusCode != http.StatusFound || reqCookie == nil {
		t.Fatalf("start = %d %v", start.StatusCode, start.Cookies())
	}

	spMeta, _ := sp.Metadata()
	var md crewjam.EntityDescriptor
	_ = xml.Unmarshal(spMeta, &md)
	idp.ServiceProviderProvider = samlSPs{&md}
	areq, err := crewjam.NewIdpAuthnRequest(idp, httptest.NewRequest("GET", start.Header.Get("Location"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := areq.Validate(); err != nil {
		t.Fatal(err)
	}
	_ = (crewjam.DefaultAssertionMaker{}).MakeAssertion(areq, samlSession{}.GetSession(nil, nil, areq))
	form, err := areq.PostBinding()
	if err != nil {
		t.Fatal(err)
	}
	post := func(withCookie bool) *http.Response {
		r, _ := http.NewRequest("POST", form.URL, strings.NewReader(url.Values{"SAMLResponse": {form.SAMLResponse}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if withCookie {
			r.AddCookie(reqCookie)
		}
		res, err := noFollow.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		spec.check(t, r, res)
		res.Body.Close()
		return res
	}
	if res := post(false); res.Header.Get("Location") != "/login?error=saml_failed" {
		t.Fatalf("without this browser's request = %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	acs := post(true)
	if acs.StatusCode != http.StatusFound || acs.Header.Get("Location") != "/" {
		t.Fatalf("acs = %d %s", acs.StatusCode, acs.Header.Get("Location"))
	}
	var session *http.Cookie
	for _, ck := range acs.Cookies() {
		if ck.Name == identityhttp.CookieName {
			session = ck
		}
	}
	if session == nil {
		t.Fatal("no session cookie")
	}
	me := c.raw("GET", "/api/v1/me", nil, session)
	defer me.Body.Close()
	if me.StatusCode != 200 {
		t.Fatalf("me = %d", me.StatusCode)
	}
}
