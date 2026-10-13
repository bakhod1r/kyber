package server_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/infrastructure/telegram"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/server"
)

// ADR-0003 leak tests: a confidential issue never reaches someone who may not see it —
// not through a read, a list, a report, an export, a notification or a Telegram group.
func TestConfidentialIssueLeaks(t *testing.T) {
	api := newBotAPI(t)
	forEachBackend(t, func(t *testing.T, anon *client) {
		api.mu.Lock()
		api.sent = nil
		api.mu.Unlock()
		lead := anon.signedIn("lead@x.uz")
		dev := anon.signedIn("dev@x.uz")
		qa := anon.signedIn("qa@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		for _, email := range []string{"dev@x.uz", "qa@x.uz"} {
			lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": email, "role": "member"})
		}
		// A Telegram group follows the project.
		_, l := lead.do("POST", "/api/v1/projects/KYB/telegram-groups/link", nil)
		api.groupStart("/start " + strings.Split(l["url"].(string), "startgroup=")[1])
		api.waitMessage(t, 1)

		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Public", "type": "task"}) // KYB-1
		api.waitMessage(t, 2)
		_, secret := dev.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Salaries 2027", "type": "task"}) // KYB-2
		api.waitMessage(t, 3)                                                                                                   // created before it was made confidential: posted, like in Jira
		code, b := dev.do("PATCH", "/api/v1/issues/KYB-2", map[string]any{"version": secret["version"], "security_level": "confidential"})
		expect(t, code, 200, b)
		if b["security_level"] != "confidential" {
			t.Fatalf("issue = %v", b)
		}
		dev.do("POST", "/api/v1/issues/KYB-2/transitions", map[string]string{"to": "in_progress"})
		dev.do("POST", "/api/v1/issues/KYB-2/comments", map[string]string{"body": "@qa@x.uz look at this"})
		time.Sleep(400 * time.Millisecond) // relay + dispatcher

		code, b = qa.do("GET", "/api/v1/issues/KYB-2", nil)
		expect(t, code, 404, b)
		_, list := qa.do("GET", "/api/v1/projects/KYB/issues", nil)
		if n := len(list["items"].([]any)); n != 1 {
			t.Fatalf("qa list has %d issues", n)
		}
		_, sum := qa.do("GET", "/api/v1/projects/KYB/reports/summary", nil)
		if sum["total"] != float64(1) {
			t.Fatalf("qa summary counts the secret issue: %v", sum["total"])
		}
		_, leadSum := lead.do("GET", "/api/v1/projects/KYB/reports/summary", nil)
		if leadSum["total"] != float64(2) {
			t.Fatalf("admin summary = %v", leadSum["total"])
		}
		for who, c := range map[string]*client{"qa": qa, "lead": lead} {
			res := c.raw("GET", "/api/v1/projects/KYB/export.csv", nil, nil)
			csv, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if strings.Contains(string(csv), "Salaries") != (who == "lead") {
				t.Fatalf("%s export:\n%s", who, csv)
			}
		}
		_, inbox := qa.do("GET", "/api/v1/notifications", nil)
		if items := inbox["items"].([]any); len(items) != 0 {
			t.Fatalf("qa was notified about a hidden issue: %v", items)
		}
		api.mu.Lock()
		defer api.mu.Unlock()
		for _, m := range api.sent {
			if strings.Contains(m, "KYB-2") && !strings.Contains(m, "created") {
				t.Fatalf("Telegram group got a confidential update: %q", m)
			}
		}
	}, server.WithTelegramBot(telegram.NewBot("TOKEN", api.URL), 50*time.Millisecond),
		server.WithSocial(identityhttp.Social{TelegramBot: "KyberBot"}),
		server.WithPublicURL("https://kyber.test"), server.WithDispatchEvery(20*time.Millisecond))
}
