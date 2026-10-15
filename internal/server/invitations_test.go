package server_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
)

var inviteLink = regexp.MustCompile(`https://kyber\.test/invite/([0-9a-f]{64})`)

// B1: a project admin invites someone without an account by email; they sign up with that address, open the
// link and become a member. Nobody else can use the link.
func TestInviteByEmail(t *testing.T) {
	mail := &mailbox{}
	forEachBackend(t, func(t *testing.T, c *client) {
		alice := c.signedIn("alice@x.uz")
		eve := c.signedIn("eve@x.uz")
		alice.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})

		code, b := eve.do("POST", "/api/v1/projects/KYB/invitations", map[string]string{"email": "dan@x.uz", "role": "member"})
		expect(t, code, 404, b) // not a member: the project does not exist for eve
		code, b = alice.do("POST", "/api/v1/projects/KYB/invitations", map[string]string{"email": "Dan@x.uz", "role": "member"})
		expect(t, code, 201, b)
		if b["added"] != false || b["emailed"] != true || b["invitation"].(map[string]any)["email"] != "dan@x.uz" {
			t.Fatalf("invite = %v", b)
		}
		link := b["link"].(string)
		token := inviteLink.FindStringSubmatch(link)
		if token == nil {
			t.Fatalf("link = %q", link)
		}
		waitFor(t, func() bool { return len(mail.to("dan@x.uz")) > 0 })
		if got := mail.to("dan@x.uz")[0]; !inviteLink.MatchString(got.Body) {
			t.Fatalf("email = %+v", got)
		}
		code, b = alice.do("GET", "/api/v1/projects/KYB/invitations", nil)
		expect(t, code, 200, b)
		if items := b["items"].([]any); len(items) != 1 {
			t.Fatalf("pending = %v", items)
		}

		// The link holder sees what they were invited to before signing in.
		code, b = c.do("GET", "/api/v1/invitations/"+token[1], nil)
		expect(t, code, 200, b)
		if b["project"] != "KYB" || b["project_name"] != "Kyber" || b["email"] != "dan@x.uz" || b["invited_by"] != "Tester" {
			t.Fatalf("preview = %v", b)
		}
		code, b = eve.do("POST", "/api/v1/invitations/"+token[1]+"/accept", nil)
		expect(t, code, 403, b) // a forwarded link does not let someone else in
		code, b = c.do("GET", "/api/v1/invitations/"+"0000", nil)
		expect(t, code, 404, b)

		dan := c.signedIn("dan@x.uz")
		code, b = dan.do("POST", "/api/v1/invitations/"+token[1]+"/accept", nil)
		expect(t, code, 200, b)
		if b["project"] != "KYB" {
			t.Fatalf("accept = %v", b)
		}
		code, b = dan.do("GET", "/api/v1/projects/KYB", nil)
		expect(t, code, 200, b)
		code, b = dan.do("POST", "/api/v1/invitations/"+token[1]+"/accept", nil)
		expect(t, code, 409, b)

		// Inviting someone who has an account adds them right away; revoking a link stops it.
		code, b = alice.do("POST", "/api/v1/projects/KYB/invitations", map[string]string{"email": "eve@x.uz", "role": "viewer"})
		expect(t, code, 200, b)
		code, b = alice.do("POST", "/api/v1/projects/KYB/invitations", map[string]string{"email": "erin@x.uz", "role": "member"})
		expect(t, code, 201, b)
		id, second := b["invitation"].(map[string]any)["id"].(string), inviteLink.FindStringSubmatch(b["link"].(string))[1]
		code, b = eve.do("DELETE", "/api/v1/projects/KYB/invitations/"+id, nil)
		expect(t, code, 403, b) // viewers cannot revoke
		code, _ = alice.do("DELETE", "/api/v1/projects/KYB/invitations/"+id, nil)
		expect(t, code, 204, nil)
		code, b = c.do("GET", "/api/v1/invitations/"+second, nil)
		expect(t, code, 404, b)
	}, server.WithMailer(mail), server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond))
}
