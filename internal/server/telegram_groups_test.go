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

func (b *botAPI) groupStart(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.pending = append(b.pending, map[string]any{"update_id": b.nextID, "message": map[string]any{"text": text,
		"from": map[string]any{"id": 777, "first_name": "Dilnoza"},
		"chat": map[string]any{"id": -100500, "type": "supergroup", "title": "Kyber devs"}}})
}

// A project admin connects a Telegram group; the group then gets the project's events.
func TestTelegramGroups(t *testing.T) {
	api := newBotAPI(t)
	forEachBackend(t, func(t *testing.T, anon *client) {
		api.mu.Lock()
		api.sent = nil
		api.mu.Unlock()
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "dev@x.uz", "role": "member"})

		code, b := dev.do("POST", "/api/v1/projects/KYB/telegram-groups/link", nil)
		expect(t, code, 403, b)
		code, b = anon.signedIn("alien@x.uz").do("POST", "/api/v1/projects/KYB/telegram-groups/link", nil)
		expect(t, code, 404, b)
		code, l := lead.do("POST", "/api/v1/projects/KYB/telegram-groups/link", nil)
		expect(t, code, 200, l)
		link, _ := url.Parse(l["url"].(string))
		token := link.Query().Get("startgroup")
		if link.Host != "t.me" || !strings.HasPrefix(token, "g_") {
			t.Fatalf("link = %v", l)
		}
		api.groupStart("/start@KyberBot " + token)
		if m := api.waitMessage(t, 1); !strings.HasPrefix(m, "-100500: ✅ This group now gets KYB updates") {
			t.Fatalf("reply = %q", m)
		}
		code, list := lead.do("GET", "/api/v1/projects/KYB/telegram-groups", nil)
		expect(t, code, 200, list)
		items := list["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["title"] != "Kyber devs" {
			t.Fatalf("groups = %v", list)
		}

		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Login", "type": "task"})
		if m := api.waitMessage(t, 2); !strings.HasPrefix(m, "-100500: 🆕 Tester created KYB-1 «Login»") {
			t.Fatalf("created = %q", m)
		}
		dev.do("POST", "/api/v1/issues/KYB-1/transitions", map[string]string{"to": "done"})
		if m := api.waitMessage(t, 3); !strings.HasPrefix(m, "-100500: 🔄 Tester moved KYB-1 «Login»: To Do → Done") ||
			!strings.Contains(m, "https://kyber.test/projects/KYB?issue=KYB-1") {
			t.Fatalf("moved = %q", m)
		}

		code, b = dev.do("DELETE", "/api/v1/projects/KYB/telegram-groups/-100500", nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", "/api/v1/projects/KYB/telegram-groups/-100500", nil)
		expect(t, code, 204, b)
		dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "After unlink", "type": "task"})
		time.Sleep(300 * time.Millisecond)
		api.mu.Lock()
		n := len(api.sent)
		api.mu.Unlock()
		if n != 3 {
			t.Fatalf("unlinked group still got messages: %d", n)
		}
	}, server.WithTelegramBot(telegram.NewBot("TOKEN", api.URL), 50*time.Millisecond),
		server.WithSocial(identityhttp.Social{TelegramBot: "KyberBot"}),
		server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond))
}
