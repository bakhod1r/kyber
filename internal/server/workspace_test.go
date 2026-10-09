package server_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/server"
)

func on(c *client, host string) *client {
	return &client{t: c.t, srv: c.srv, token: c.token, host: host}
}

func TestS33Workspaces(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		apex := on(anon, "kyber.test")
		alice := apex.signedIn("alice@x.uz")
		bob := apex.signedIn("bob@x.uz")
		eve := apex.signedIn("eve@x.uz")

		// AC1 workspaces are created on the apex; the creator owns them.
		code, ws := alice.do("POST", "/api/v1/workspaces", map[string]string{"slug": "acme", "name": "Acme"})
		expect(t, code, 201, ws)
		if ws["url"] != "http://acme.kyber.test/" || ws["role"] != "owner" {
			t.Fatalf("workspace = %v", ws)
		}
		code, b := bob.do("POST", "/api/v1/workspaces", map[string]string{"slug": "acme", "name": "Again"})
		expect(t, code, 409, b)
		code, b = bob.do("POST", "/api/v1/workspaces", map[string]string{"slug": "www", "name": "W"})
		expect(t, code, 422, b)
		bob.do("POST", "/api/v1/workspaces", map[string]string{"slug": "globex", "name": "Globex"})
		code, mine := alice.do("GET", "/api/v1/workspaces", nil)
		if code != 200 || len(mine["items"].([]any)) != 1 {
			t.Fatalf("mine = %v", mine)
		}

		// AC2 projects live on the workspace's subdomain.
		acme := on(alice, "acme.kyber.test")
		code, p := acme.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		expect(t, code, 201, p)
		acme.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Secret", "type": "task"})

		// AC3 isolation: the project does not exist on another workspace, even for its admin.
		globexAsAlice := on(alice, "globex.kyber.test")
		for _, path := range []string{"/api/v1/projects/KYB", "/api/v1/issues/KYB-1", "/api/v1/projects/KYB/reports/summary"} {
			code, b := globexAsAlice.do("GET", path, nil)
			expect(t, code, 404, b)
		}
		code, list := on(bob, "globex.kyber.test").do("GET", "/api/v1/projects", nil)
		if code != 200 || len(list["items"].([]any)) != 0 {
			t.Fatalf("globex projects = %v", list)
		}
		// AC4 non-members of a workspace see nothing there.
		code, b = on(eve, "acme.kyber.test").do("GET", "/api/v1/projects", nil)
		expect(t, code, 404, b)
		// AC5 adding someone to a project lets them into the workspace.
		acme.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "eve@x.uz", "role": "viewer"})
		code, b = on(eve, "acme.kyber.test").do("GET", "/api/v1/issues/KYB-1", nil)
		expect(t, code, 200, b)

		// AC6 the apex and unknown hosts serve no workspace API.
		code, b = alice.do("GET", "/api/v1/projects", nil)
		expect(t, code, 404, b)
		code, b = on(alice, "nope.kyber.test").do("GET", "/api/v1/projects", nil)
		expect(t, code, 404, b)
		code, b = on(alice, "evil.example.com").do("GET", "/api/v1/me", nil)
		expect(t, code, 404, b)
		if code, _ := on(anon, "10.0.0.1").do("GET", "/healthz", nil); code != 200 {
			t.Fatalf("probes answer on any host: %d", code)
		}
	}, server.WithBaseDomain("kyber.test"))
}
