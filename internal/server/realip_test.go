package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/netip"
	"testing"

	"github.com/bakhod1r/kyber/internal/platform/realip"
	"github.com/bakhod1r/kyber/internal/server"
)

// loginFrom sends a login as if it came through a reverse proxy for the client at xff.
func (c *client) loginFrom(xff, password string) int {
	c.t.Helper()
	b, _ := json.Marshal(map[string]string{"email": "t@x.uz", "password": password})
	req, _ := http.NewRequest("POST", c.srv.URL+"/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", xff)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// Behind a trusted reverse proxy, the login throttle counts the real client (X-Forwarded-For), not the
// proxy: one attacker no longer locks everyone else out of an account (QA-03 limitation 2).
func TestTrustedProxyThrottlesTheRealClient(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "t@x.uz", "name": "T", "password": "long enough pw"})
		for range 10 {
			if code := c.loginFrom("203.0.113.7", "wrong password"); code != 401 {
				t.Fatalf("attacker login = %d", code)
			}
		}
		if code := c.loginFrom("203.0.113.7", "long enough pw"); code != 429 {
			t.Fatalf("attacker after 10 attempts = %d, want 429", code)
		}
		if code := c.loginFrom("198.51.100.9", "long enough pw"); code != 200 {
			t.Fatalf("another client behind the proxy = %d, want 200", code)
		}
		// A client cannot dodge the throttle by prepending its own X-Forwarded-For entry.
		if code := c.loginFrom("198.51.100.9, 203.0.113.7", "long enough pw"); code != 429 {
			t.Fatalf("spoofed chain = %d, want 429", code)
		}
	}, server.WithTrustedProxies(mustProxies(t, "127.0.0.0/8, ::1/128")))
}

// Without trusted proxies X-Forwarded-For is ignored: a direct client cannot pick its own IP.
func TestUntrustedForwardedForIsIgnored(t *testing.T) {
	forEachBackend(t, func(t *testing.T, c *client) {
		c.do("POST", "/api/v1/auth/signup", map[string]string{"email": "t@x.uz", "name": "T", "password": "long enough pw"})
		for range 10 {
			c.loginFrom("203.0.113.7", "wrong password")
		}
		if code := c.loginFrom("198.51.100.9", "long enough pw"); code != 429 {
			t.Fatalf("forged X-Forwarded-For = %d, want 429", code)
		}
	})
}

func mustProxies(t *testing.T, csv string) []netip.Prefix {
	p, err := realip.Parse(csv)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
