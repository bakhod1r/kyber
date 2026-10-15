package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// ADR-0016: an MCP client discovers the authorization server, registers itself, the user approves
// it, and the code (PKCE S256) becomes an access token that works on /api/v1/mcp.
func TestMCPOAuth(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		// Unauthenticated MCP requests point at the protected resource metadata.
		res := anon.raw("POST", "/api/v1/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}, nil)
		res.Body.Close()
		if res.StatusCode != 401 || !strings.Contains(res.Header.Get("WWW-Authenticate"), `resource_metadata="`+anon.srv.URL+`/.well-known/oauth-protected-resource"`) {
			t.Fatalf("challenge = %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
		}
		code, prm := anon.do("GET", "/.well-known/oauth-protected-resource", nil)
		expect(t, code, 200, prm)
		if prm["resource"] != anon.srv.URL+"/api/v1/mcp" || prm["authorization_servers"].([]any)[0] != anon.srv.URL {
			t.Fatalf("resource metadata = %v", prm)
		}
		if code, again := anon.do("GET", "/.well-known/oauth-protected-resource/api/v1/mcp", nil); code != 200 || again["resource"] != prm["resource"] {
			t.Fatalf("path-suffixed metadata = %d %v", code, again)
		}
		code, b := anon.do("GET", "/api/v1/oauth/clients/nope", nil)
		expect(t, code, 401, b)
		code, meta := anon.do("GET", "/.well-known/oauth-authorization-server", nil)
		expect(t, code, 200, meta)
		if meta["authorization_endpoint"] != anon.srv.URL+"/oauth/authorize" || meta["token_endpoint"] != anon.srv.URL+"/api/v1/oauth/token" ||
			meta["registration_endpoint"] != anon.srv.URL+"/api/v1/oauth/register" || meta["code_challenge_methods_supported"].([]any)[0] != "S256" {
			t.Fatalf("server metadata = %v", meta)
		}

		code, reg := anon.do("POST", "/api/v1/oauth/register", map[string]any{"client_name": "Claude", "redirect_uris": []string{"http://localhost:7777/callback"}})
		expect(t, code, 201, reg)
		clientID := reg["client_id"].(string)
		code, b = anon.do("POST", "/api/v1/oauth/register", map[string]any{"client_name": "Evil", "redirect_uris": []string{"http://evil.example/cb"}})
		expect(t, code, 400, b)
		if b["error"] != "invalid_redirect_uri" {
			t.Fatalf("registration error = %v", b)
		}

		ali := anon.signedIn("ali@x.uz")
		code, shown := ali.do("GET", "/api/v1/oauth/clients/"+clientID, nil)
		expect(t, code, 200, shown)
		if shown["client_name"] != "Claude" {
			t.Fatalf("consent client = %v", shown)
		}
		code, b = ali.do("GET", "/api/v1/oauth/clients/96000000-0000-4000-8000-0000000000ff", nil)
		expect(t, code, 404, b)
		verifier := strings.Repeat("k", 64)
		sum := sha256.Sum256([]byte(verifier))
		approve := map[string]string{"client_id": clientID, "redirect_uri": "http://localhost:51234/callback", "code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]),
			"code_challenge_method": "S256", "state": "xyz"}
		code, out := ali.do("POST", "/api/v1/oauth/authorize", approve)
		expect(t, code, 200, out)
		to, _ := url.Parse(out["redirect_to"].(string))
		if to.Host != "localhost:51234" || to.Query().Get("state") != "xyz" || to.Query().Get("code") == "" {
			t.Fatalf("redirect = %v", out)
		}
		approve["redirect_uri"] = "https://evil.example/cb"
		code, b = ali.do("POST", "/api/v1/oauth/authorize", approve)
		expect(t, code, 422, b)
		// Consent must be JSON (no simple cross-site form or text posts), even with a session.
		approve["redirect_uri"] = "http://localhost:51234/callback"
		body, _ := json.Marshal(approve)
		plain, _ := http.NewRequest("POST", anon.srv.URL+"/api/v1/oauth/authorize", bytes.NewReader(body))
		plain.Header.Set("Content-Type", "text/plain")
		plain.Header.Set("Authorization", "Bearer "+ali.token)
		res, err := http.DefaultClient.Do(plain)
		if err != nil {
			t.Fatal(err)
		}
		spec.check(t, plain, res)
		res.Body.Close()
		if res.StatusCode != 400 {
			t.Fatalf("non-JSON consent = %d", res.StatusCode)
		}

		exchange := func(code, verifier string) (int, map[string]any) {
			form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
				"redirect_uri": {"http://localhost:51234/callback"}, "code_verifier": {verifier}}
			req, _ := http.NewRequest("POST", anon.srv.URL+"/api/v1/oauth/token", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			spec.check(t, req, res)
			defer res.Body.Close()
			var body map[string]any
			_ = json.NewDecoder(res.Body).Decode(&body)
			if res.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("token responses must not be cached")
			}
			return res.StatusCode, body
		}
		code, b = exchange(to.Query().Get("code"), strings.Repeat("x", 64))
		expect(t, code, 400, b)
		if b["error"] != "invalid_grant" {
			t.Fatalf("wrong verifier = %v", b)
		}
		_, out = ali.do("POST", "/api/v1/oauth/authorize", map[string]string{"client_id": clientID, "redirect_uri": "http://localhost:51234/callback",
			"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256"})
		to, _ = url.Parse(out["redirect_to"].(string))
		code, tok := exchange(to.Query().Get("code"), verifier)
		expect(t, code, 200, tok)
		if tok["token_type"] != "Bearer" || !strings.HasPrefix(tok["access_token"].(string), "kpat_") || tok["expires_in"] != float64(90*24*3600) {
			t.Fatalf("token = %v", tok)
		}
		mcp := &client{t: t, srv: anon.srv, token: tok["access_token"].(string)}
		code, b = mcp.do("POST", "/api/v1/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
		expect(t, code, 200, b)
		// The access token is a personal access token and cannot approve further clients.
		code, b = mcp.do("POST", "/api/v1/oauth/authorize", map[string]string{"client_id": clientID, "redirect_uri": "http://localhost:51234/callback",
			"code_challenge": base64.RawURLEncoding.EncodeToString(sum[:]), "code_challenge_method": "S256"})
		expect(t, code, 403, b)
	})
}

// ADR-0016: anonymous registration is limited per client IP (10 per 15 minutes).
func TestMCPOAuthRegistrationRateLimit(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		reg := map[string]any{"client_name": "Bot", "redirect_uris": []string{"http://localhost:7777/callback"}}
		for range 10 {
			code, b := anon.do("POST", "/api/v1/oauth/register", reg)
			expect(t, code, 201, b)
		}
		res := anon.raw("POST", "/api/v1/oauth/register", reg, nil)
		defer res.Body.Close()
		var b map[string]any
		_ = json.NewDecoder(res.Body).Decode(&b)
		expect(t, res.StatusCode, 429, b)
		if ra := res.Header.Get("Retry-After"); ra == "" || ra == "0" {
			t.Fatalf("Retry-After = %q", ra)
		}
		if b["error"] != "too_many_requests" || b["error_description"] == "" {
			t.Fatalf("body = %v", b)
		}
	})
}
