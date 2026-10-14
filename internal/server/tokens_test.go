package server_test

import (
	"net/http"
	"testing"
)

// Personal access tokens (ADR-0012): long-lived credentials for scripts and MCP, managed only
// from a signed-in session.
func TestPersonalAccessTokens(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		ann := anon.signedIn("ann@x.uz")
		ben := anon.signedIn("ben@x.uz")
		ann.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})

		code, b := ann.do("POST", "/api/v1/me/tokens", map[string]any{"name": "", "expires_in_days": 30})
		expect(t, code, 422, b)
		code, b = ann.do("POST", "/api/v1/me/tokens", map[string]any{"name": "x", "expires_in_days": 400})
		expect(t, code, 422, b)
		code, tok := ann.do("POST", "/api/v1/me/tokens", map[string]any{"name": "Claude Code", "expires_in_days": 30})
		expect(t, code, 201, tok)
		secret, _ := tok["token"].(string)
		if len(secret) < 40 || secret[:5] != "kpat_" || tok["expires_at"] == nil || tok["last_used_at"] != nil {
			t.Fatalf("token = %v", tok)
		}

		bot := &client{t: t, srv: ann.srv, host: ann.host, token: secret}
		code, me := bot.do("GET", "/api/v1/me", nil)
		expect(t, code, 200, me)
		if me["email"] != "ann@x.uz" {
			t.Fatalf("me = %v", me)
		}
		code, b = bot.do("GET", "/api/v1/projects/KYB", nil)
		expect(t, code, 200, b)
		for _, req := range []struct{ method, path string }{{"GET", "/api/v1/me/tokens"}, {"POST", "/api/v1/me/tokens"}, {"DELETE", "/api/v1/me/tokens/" + tok["id"].(string)}} {
			code, b = bot.do(req.method, req.path, map[string]any{"name": "child"})
			expect(t, code, http.StatusForbidden, b)
		}

		code, list := ann.do("GET", "/api/v1/me/tokens", nil)
		expect(t, code, 200, list)
		items := list["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["last_used_at"] == nil || items[0].(map[string]any)["token"] != nil {
			t.Fatalf("list = %v", list)
		}
		code, b = ben.do("DELETE", "/api/v1/me/tokens/"+tok["id"].(string), nil)
		expect(t, code, 404, b)
		code, b = ann.do("DELETE", "/api/v1/me/tokens/"+tok["id"].(string), nil)
		expect(t, code, 204, b)
		code, b = bot.do("GET", "/api/v1/me", nil)
		expect(t, code, 401, b)
	})
}
