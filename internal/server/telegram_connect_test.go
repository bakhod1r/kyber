package server_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/infrastructure/telegram"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/server"
)

// Connect Telegram from the settings page, then get notifications in the bot chat.
func TestTelegramConnectAndNotifications(t *testing.T) {
	api := newBotAPI(t)
	forEachBackend(t, func(t *testing.T, anon *client) {
		api.mu.Lock()
		api.sent = nil
		api.mu.Unlock()
		lead := anon.signedIn("lead@x.uz")
		bob := anon.signedIn("bob@x.uz")
		_, me := bob.do("GET", "/api/v1/me", nil)
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "bob@x.uz", "role": "member"})

		code, c := bob.do("POST", "/api/v1/me/telegram/connect", nil)
		expect(t, code, 200, c)
		link, _ := url.Parse(c["url"].(string))
		start := link.Query().Get("start")
		if link.Host != "t.me" || link.Path != "/KyberBot" || !strings.HasPrefix(start, "c_") || c["expires_at"] == nil {
			t.Fatalf("link = %v", c)
		}
		api.start("/start " + start)
		if m := api.waitMessage(t, 1); !strings.HasPrefix(m, "7770: ✅ Telegram connected") || !strings.Contains(m, "bob@x.uz") {
			t.Fatalf("reply = %q", m)
		}
		code, s := bob.do("GET", "/api/v1/me/notification-settings", nil)
		expect(t, code, 200, s)
		if s["telegram_connected"] != true || s["telegram_available"] != true {
			t.Fatalf("settings = %v", s)
		}
		// The same link cannot be used again.
		api.start("/start " + start)
		if m := api.waitMessage(t, 2); !strings.Contains(m, "not valid any more") {
			t.Fatalf("reuse reply = %q", m)
		}

		_, is := lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		lead.do("PATCH", "/api/v1/issues/KYB-1", map[string]any{"version": is["version"], "assignee_id": me["id"]})
		if m := api.waitMessage(t, 3); !strings.HasPrefix(m, "777: Tester assigned you KYB-1 «Login».") ||
			!strings.Contains(m, "https://kyber.test/projects/KYB?issue=KYB-1") {
			t.Fatalf("notification = %q", m)
		}
	}, server.WithTelegramBot(telegram.NewBot("TOKEN", api.URL), 50*time.Millisecond),
		server.WithSocial(identityhttp.Social{TelegramBot: "KyberBot"}),
		server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond))
}
