package server_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/server"
)

// zonePastDigestHour is a fixed-offset zone where it is now between 09:00 and 22:59, so the digest is due.
func zonePastDigestHour(t *testing.T) string {
	for off := -12; off <= 14; off++ {
		name := fmt.Sprintf("Etc/GMT%+d", -off) // Etc/GMT-5 is UTC+5
		if off == 0 {
			name = "Etc/UTC"
		}
		loc, err := time.LoadLocation(name)
		if err != nil {
			continue
		}
		if h := time.Now().In(loc).Hour(); h >= 9 && h <= 22 {
			return name
		}
	}
	t.Fatal("no zone found")
	return ""
}

// B9: with the daily digest on, notifications are not emailed one by one; one digest email lists them.
func TestDailyDigestEmail(t *testing.T) {
	mail := &mailbox{}
	forEachBackend(t, func(t *testing.T, anon *client) {
		mail.mu.Lock()
		mail.sent = nil
		mail.mu.Unlock()
		lead := anon.signedIn("lead@x.uz")
		bob := anon.signedIn("bob@x.uz")
		_, me := bob.do("GET", "/api/v1/me", nil)
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "member"})

		code, s := bob.do("PUT", "/api/v1/me/notification-settings", map[string]any{"email_digest": true, "time_zone": zonePastDigestHour(t)})
		expect(t, code, 200, s)
		if s["email_digest"] != true || s["email"] != true {
			t.Fatalf("settings = %v", s)
		}
		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "assignee_id": me["id"]})
		waitFor(t, func() bool { return len(mail.to("bob@x.uz")) == 1 })
		got := mail.to("bob@x.uz")[0]
		if got.Subject != "Kyber: 1 unread notification" || !strings.Contains(got.Body, "Tester assigned you") ||
			!strings.Contains(got.Body, "https://kyber.test/projects/KYB?issue=KYB-1") {
			t.Fatalf("digest = %+v", got)
		}
		time.Sleep(200 * time.Millisecond) // the job keeps running: still one digest today
		if n := len(mail.to("bob@x.uz")); n != 1 {
			t.Fatalf("%d digests", n)
		}
	}, server.WithMailer(mail), server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond),
		server.WithReminderEvery(20*time.Millisecond))
}
