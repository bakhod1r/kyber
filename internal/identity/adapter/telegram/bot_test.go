package telegram_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/telegram"
)

// fakeAPI is a Bot API stub: it serves queued updates once and records sent messages.
type fakeAPI struct {
	*httptest.Server
	mu      sync.Mutex
	updates []map[string]any
	sent    []map[string]any
	offsets []float64
	failGet int // fail this many getUpdates calls first
}

func newAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case !strings.HasPrefix(r.URL.Path, "/botTOKEN/"):
			_, _ = io.WriteString(w, `{"ok":false,"description":"Unauthorized"}`)
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			f.sent = append(f.sent, in)
			_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			if f.failGet > 0 {
				f.failGet--
				_, _ = io.WriteString(w, `not json`)
				return
			}
			f.offsets = append(f.offsets, in["offset"].(float64))
			out, _ := json.Marshal(map[string]any{"ok": true, "result": f.updates})
			f.updates = nil
			_, _ = w.Write(out)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func msg(id int, chatType, text string) map[string]any {
	return map[string]any{"update_id": id, "message": map[string]any{"text": text,
		"from": map[string]any{"id": 42, "first_name": "Bobur", "username": "bobur"}, "chat": map[string]any{"id": 4200, "type": chatType}}}
}

func TestSendMessage(t *testing.T) {
	api := newAPI(t)
	if err := telegram.NewBot("TOKEN", api.URL).SendMessage(context.Background(), "4200", "hi"); err != nil {
		t.Fatal(err)
	}
	if len(api.sent) != 1 || api.sent[0]["chat_id"] != "4200" || api.sent[0]["text"] != "hi" {
		t.Fatalf("sent = %v", api.sent)
	}
	if err := telegram.NewBot("WRONG", api.URL).SendMessage(context.Background(), "1", "x"); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("api error = %v", err)
	}
	err := telegram.NewBot("SECRET-TOKEN", "http://127.0.0.1:1").SendMessage(context.Background(), "1", "x")
	if err == nil || strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("transport error must not leak the token: %v", err)
	}
	if err := telegram.NewBot("TOKEN", "http://[::1]:namedport").SendMessage(context.Background(), "1", "x"); err == nil {
		t.Fatal("bad base URL must fail")
	}
	if telegram.NewBot("t", "") == nil {
		t.Fatal("default base")
	}
}

func TestPoll(t *testing.T) {
	api := newAPI(t)
	api.failGet = 1
	api.updates = []map[string]any{
		msg(10, "private", "/start nonce-1"),
		msg(11, "group", "/start nonce-group"), // ignored: not a private chat
		msg(12, "private", "hello"),            // ignored: not /start
		{"update_id": 13},                      // ignored: no message
		msg(14, "private", "/start bad"),       // handler error is logged, polling continues
	}
	ctx, cancel := context.WithCancel(context.Background())
	var got []string
	done := make(chan struct{})
	go func() {
		telegram.NewBot("TOKEN", api.URL).Poll(ctx, slog.New(slog.DiscardHandler), 0, time.Millisecond,
			func(_ context.Context, payload string, from telegram.User, chat string) error {
				got = append(got, payload+"|"+from.DisplayName()+"|"+chat)
				if payload == "bad" {
					return context.DeadlineExceeded
				}
				return nil
			})
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		api.mu.Lock()
		n := len(api.offsets)
		api.mu.Unlock()
		if n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if strings.Join(got, ",") != "nonce-1|Bobur|4200,bad|Bobur|4200" {
		t.Fatalf("handled = %v", got)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.offsets[0] != 0 || api.offsets[1] != 15 {
		t.Fatalf("offsets = %v (must acknowledge processed updates)", api.offsets)
	}
}

func TestPollStopsWhileBackingOff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		telegram.NewBot("T", "http://127.0.0.1:1").Poll(ctx, slog.New(slog.DiscardHandler), 0, time.Hour, nil)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poll did not stop")
	}
}

func TestDisplayName(t *testing.T) {
	if (telegram.User{Username: "solo"}).DisplayName() != "solo" || (telegram.User{FirstName: "A", LastName: "B"}).DisplayName() != "A B" {
		t.Fatal("display name")
	}
}
