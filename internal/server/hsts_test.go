package server_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/server"
)

// Behind HTTPS (secure cookies) browsers are told to stay on HTTPS; plain-HTTP dev mode is not.
func TestHSTS(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	secure := httptest.NewServer(server.NewPostgres(log, dbtest.New(t), true, server.WithContext(t.Context())))
	defer secure.Close()
	plain := httptest.NewServer(server.NewInMemory(log, server.WithContext(t.Context())))
	defer plain.Close()
	for _, path := range []string{"/", "/healthz", "/api/v1/me"} {
		c := &client{t: t, srv: secure}
		res := c.raw("GET", path, nil, nil)
		res.Body.Close()
		if got := res.Header.Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
			t.Errorf("secure %s: HSTS = %q", path, got)
		}
		p := &client{t: t, srv: plain}
		res = p.raw("GET", path, nil, nil)
		res.Body.Close()
		if got := res.Header.Get("Strict-Transport-Security"); got != "" {
			t.Errorf("plain %s: HSTS = %q", path, got)
		}
	}
}
