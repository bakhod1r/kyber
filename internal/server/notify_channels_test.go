package server_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	notifydomain "github.com/bakhod1r/kyber/internal/notify/domain"
	"github.com/bakhod1r/kyber/internal/server"
)

type mailbox struct {
	mu   sync.Mutex
	sent []notifydomain.Delivery
}

func (m *mailbox) Send(_ context.Context, d notifydomain.Delivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, d)
	return nil
}

func (m *mailbox) to(addr string) []notifydomain.Delivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []notifydomain.Delivery
	for _, d := range m.sent {
		if d.To == addr {
			out = append(out, d)
		}
	}
	return out
}

// Notifications are emailed (through the configured SMTP mailer) unless the user opts out.
func TestEmailNotifications(t *testing.T) {
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

		code, s := bob.do("GET", "/api/v1/me/notification-settings", nil)
		expect(t, code, 200, s)
		if s["email"] != true || s["email_available"] != true || s["telegram_available"] != false || s["telegram_connected"] != false {
			t.Fatalf("settings = %v", s)
		}
		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "assignee_id": me["id"]})
		waitFor(t, func() bool { return len(mail.to("bob@x.uz")) == 1 })
		got := mail.to("bob@x.uz")[0]
		if got.Subject != "[KYB-1] Tester assigned you: Login" || !strings.Contains(got.Body, "https://kyber.test/projects/KYB?issue=KYB-1") {
			t.Fatalf("email = %+v", got)
		}

		code, s = bob.do("PUT", "/api/v1/me/notification-settings", map[string]any{"email": false})
		expect(t, code, 200, s)
		if s["email"] != false || s["telegram"] != true {
			t.Fatalf("saved = %v", s)
		}
		lead.do("POST", "/api/v1/issues/KYB-1/comments", map[string]string{"body": "ping"})
		time.Sleep(300 * time.Millisecond) // relay + dispatcher run every 20 ms
		if n := len(mail.to("bob@x.uz")); n != 1 {
			t.Fatalf("opted-out user got %d emails", n)
		}

		// Meeting times in messages follow the recipient's browser time zone.
		code, s = bob.do("PUT", "/api/v1/me/notification-settings", map[string]any{"time_zone": "Mars/Base"})
		expect(t, code, 422, s)
		code, s = bob.do("PUT", "/api/v1/me/notification-settings", map[string]any{"email": true, "time_zone": "Asia/Tashkent"})
		if code != 200 || s["time_zone"] != "Asia/Tashkent" || s["email"] != true {
			t.Fatalf("time zone = %d %v", code, s)
		}
		start := time.Date(time.Now().Year()+1, 3, 2, 10, 0, 0, 0, time.UTC)
		_, m := lead.do("POST", "/api/v1/meetings", map[string]any{"title": "Demo", "starts_at": start, "ends_at": start.Add(time.Hour),
			"attendee_ids": []any{me["id"]}})
		code, m = lead.do("PUT", "/api/v1/meetings/"+m["id"].(string), map[string]any{"title": "Demo", "starts_at": start.Add(time.Hour),
			"ends_at": start.Add(2 * time.Hour)})
		expect(t, code, 200, m)
		waitFor(t, func() bool {
			for _, e := range mail.to("bob@x.uz") {
				if strings.Contains(e.Subject, "Moved to") && strings.Contains(e.Subject, "16:00–17:00 Asia/Tashkent") {
					return true
				}
			}
			return false
		})
	}, server.WithMailer(mail), server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond))
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("condition not met in 5 s")
}
