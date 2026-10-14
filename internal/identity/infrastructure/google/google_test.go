package google_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/google"
)

// The OpenID Connect flow itself is tested in oidcprovider; this checks Google's preset.
func TestPreset(t *testing.T) {
	if google.New(google.Config{}).Enabled() || !google.New(google.Config{ClientID: "c", ClientSecret: "s"}).Enabled() {
		t.Fatal("enabled only with client id and secret (the issuer defaults to Google's)")
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/auth",
			"token_endpoint": srv.URL + "/token", "jwks_uri": srv.URL + "/jwks"})
	}))
	t.Cleanup(srv.Close)
	c := google.New(google.Config{Issuer: srv.URL, ClientID: "c", ClientSecret: "s", RedirectURL: "https://kyber.test/cb"})
	u, err := c.AuthURL(context.Background(), application.NewOIDCFlow())
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	if q.Query().Get("prompt") != "select_account" || q.Query().Get("client_id") != "c" {
		t.Fatalf("auth url = %s", u)
	}
}
