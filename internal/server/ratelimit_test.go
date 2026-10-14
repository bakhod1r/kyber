package server_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

// Each signed-in user gets a request budget per minute; over it the API answers 429 RATE_LIMITED.
func TestAPIRateLimit(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		ali := anon.signedIn("ali@x.uz")
		bob := anon.signedIn("bob@x.uz")
		for range 3 {
			code, b := ali.do("GET", "/api/v1/projects", nil)
			expect(t, code, 200, b)
		}
		code, b := ali.do("GET", "/api/v1/projects", nil)
		expect(t, code, 429, b)
		if b["code"] != "RATE_LIMITED" {
			t.Fatalf("problem = %v", b)
		}
		code, b = bob.do("GET", "/api/v1/projects", nil)
		expect(t, code, 200, b)
	}, server.WithRateLimits(3, 1))
}
