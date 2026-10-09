package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	identityhttp "github.com/bakhod1r/kyber/internal/identity/adapter/httpapi"
	"github.com/bakhod1r/kyber/internal/identity/adapter/telegram"
	"github.com/bakhod1r/kyber/internal/server"
)

// botAPI fakes the Telegram Bot API: updates are queued by the test, messages are recorded.
type botAPI struct {
	*httptest.Server
	mu      sync.Mutex
	pending []map[string]any
	sent    []string
	nextID  int
}

func newBotAPI(t *testing.T) *botAPI {
	b := &botAPI{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			b.mu.Lock()
			b.sent = append(b.sent, in["chat_id"].(string)+": "+in["text"].(string))
			b.mu.Unlock()
			_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
			return
		}
		time.Sleep(10 * time.Millisecond) // a short long-poll
		b.mu.Lock()
		out, _ := json.Marshal(map[string]any{"ok": true, "result": b.pending})
		b.pending = nil
		b.mu.Unlock()
		_, _ = w.Write(out)
	}))
	t.Cleanup(b.Close)
	return b
}

func (b *botAPI) start(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	b.pending = append(b.pending, map[string]any{"update_id": b.nextID, "message": map[string]any{"text": text,
		"from": map[string]any{"id": 777, "first_name": "Dilnoza"}, "chat": map[string]any{"id": 7770, "type": "private"}}})
}

func (b *botAPI) waitMessage(t *testing.T, n int) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		if len(b.sent) >= n {
			m := b.sent[n-1]
			b.mu.Unlock()
			return m
		}
		b.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("bot sent %d messages, want %d", len(b.sent), n)
	return ""
}

func TestS40TelegramOTP(t *testing.T) {
	api := newBotAPI(t)
	opts := []server.Option{
		server.WithSocial(identityhttp.Social{TelegramBot: "kyber_bot"}),
		server.WithTelegramBot(telegram.NewBot("TOKEN", api.URL), 0),
	}
	sentBefore := 0
	forEachBackend(t, func(t *testing.T, anon *client) {
		code, p := anon.do("GET", "/api/v1/auth/providers", nil)
		expect(t, code, 200, p)
		if p["telegram_otp"] != true {
			t.Fatalf("providers = %v", p)
		}
		// AC1 the site returns a deep link to the bot.
		code, ch := anon.do("POST", "/api/v1/auth/telegram/otp", map[string]any{})
		expect(t, code, 201, ch)
		link := ch["link"].(string)
		if !strings.HasPrefix(link, "https://t.me/kyber_bot?start=") || ch["id"] == "" {
			t.Fatalf("challenge = %v", ch)
		}
		// Before Start is pressed there is no code yet.
		code, b := anon.do("POST", "/api/v1/auth/telegram/otp/verify", map[string]any{"id": ch["id"], "code": "123456"})
		expect(t, code, 409, b)

		// AC2 pressing Start makes the bot send a code to that chat.
		api.start("/start " + strings.TrimPrefix(link, "https://t.me/kyber_bot?start="))
		sentBefore++
		msg := api.waitMessage(t, sentBefore)
		got := regexp.MustCompile(`\b\d{6}\b`).FindString(msg)
		if !strings.HasPrefix(msg, "7770: Your Kyber login code: ") || got == "" {
			t.Fatalf("bot message = %q", msg)
		}
		// AC3 a wrong code is refused; the right one signs in.
		wrong := "000000"
		if got == wrong {
			wrong = "111111"
		}
		code, b = anon.do("POST", "/api/v1/auth/telegram/otp/verify", map[string]any{"id": ch["id"], "code": wrong})
		expect(t, code, 401, b)
		code, tok := anon.do("POST", "/api/v1/auth/telegram/otp/verify", map[string]any{"id": ch["id"], "code": got})
		expect(t, code, 200, tok)
		me := &client{t: t, srv: anon.srv, token: tok["token"].(string)}
		if code, u := me.do("GET", "/api/v1/me", nil); code != 200 || u["name"] != "Dilnoza" {
			t.Fatalf("me = %v", u)
		}
		// AC4 single use; a stale /start gets instructions.
		code, b = anon.do("POST", "/api/v1/auth/telegram/otp/verify", map[string]any{"id": ch["id"], "code": got})
		expect(t, code, 401, b)
		api.start("/start no-such-nonce")
		sentBefore++
		if m := api.waitMessage(t, sentBefore); !strings.Contains(m, "not valid any more") {
			t.Fatalf("stale start reply = %q", m)
		}
	}, opts...)
}

func TestS40TelegramOTPDisabled(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		code, b := anon.do("POST", "/api/v1/auth/telegram/otp", map[string]any{})
		expect(t, code, 404, b)
		code, b = anon.do("POST", "/api/v1/auth/telegram/otp/verify", map[string]any{"id": "x", "code": "1"})
		expect(t, code, 404, b)
	})
}

// Regression (review): a cross-site <form enctype="text/plain"> must not drive the OTP login
// (login CSRF: the victim would be signed into the attacker's account).
func TestS40OTPRejectsNonJSON(t *testing.T) {
	api := newBotAPI(t)
	forEachBackend(t, func(t *testing.T, anon *client) {
		for _, path := range []string{"/api/v1/auth/telegram/otp", "/api/v1/auth/telegram/otp/verify"} {
			req, _ := http.NewRequest("POST", anon.srv.URL+path, strings.NewReader(`{"id":"x","code":"123456"}`))
			req.Header.Set("Content-Type", "text/plain")
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			spec.check(t, req, res)
			res.Body.Close()
			if res.StatusCode != http.StatusForbidden {
				t.Fatalf("%s with text/plain = %d, want 403", path, res.StatusCode)
			}
		}
	}, server.WithSocial(identityhttp.Social{TelegramBot: "kyber_bot"}), server.WithTelegramBot(telegram.NewBot("TOKEN", api.URL), 0))
}
