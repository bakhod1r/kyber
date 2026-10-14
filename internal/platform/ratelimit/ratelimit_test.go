package ratelimit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	guard "github.com/bakhod1r/guard/ratelimit"

	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/ratelimit"
)

func TestMemorySlidingWindow(t *testing.T) {
	now := time.Unix(0, 0)
	m := ratelimit.NewMemory(func() time.Time { return now })
	rule := guard.Rule{Limit: 2, Window: time.Minute}
	ctx := context.Background()
	for i, want := range []bool{true, true, false} {
		res, err := m.Allow(ctx, "u", rule)
		if err != nil || res.Allowed != want {
			t.Fatalf("hit %d = %+v %v", i, res, err)
		}
	}
	if res, _ := m.Allow(ctx, "other", rule); !res.Allowed || res.Remaining != 1 {
		t.Fatalf("keys are independent: %+v", res)
	}
	res, _ := m.Allow(ctx, "u", rule)
	if res.RetryAfter != time.Minute {
		t.Fatalf("retry after = %v", res.RetryAfter)
	}
	now = now.Add(time.Minute + time.Second)
	if res, _ := m.Allow(ctx, "u", rule); !res.Allowed || res.Remaining != 1 {
		t.Fatalf("after the window: %+v", res)
	}
	// Idle keys are swept now and then; a swept key starts afresh.
	now = now.Add(time.Hour)
	for i := range 1100 {
		_, _ = m.Allow(ctx, string(rune('a'+i%26))+time.Duration(i).String(), rule)
	}
	if res, _ := m.Allow(ctx, "u", rule); !res.Allowed || res.Remaining != 1 {
		t.Fatalf("after the sweep: %+v", res)
	}
	if _, err := m.Allow(ctx, "u", guard.Rule{}); err == nil {
		t.Fatal("invalid rule accepted")
	}
}

type failing struct{}

func (failing) Allow(context.Context, string, guard.Rule) (guard.Result, error) {
	return guard.Result{}, errors.New("redis down")
}

func TestMiddleware(t *testing.T) {
	now := time.Unix(0, 0)
	lim := ratelimit.NewMemory(func() time.Time { return now })
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := ratelimit.Middleware(lim, ratelimit.Rules{API: guard.Rule{Limit: 2, Window: time.Minute}, Upload: guard.Rule{Limit: 1, Window: time.Minute}}, log)(ok)
	do := func(h http.Handler, method, path, user string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if user != "" {
			r = r.WithContext(auth.WithActor(r.Context(), user))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for i := range 2 {
		if w := do(h, "GET", "/api/v1/projects", "u-1"); w.Code != 204 || w.Header().Get("X-RateLimit-Limit") != "2" {
			t.Fatalf("request %d = %d %v", i, w.Code, w.Header())
		}
	}
	w := do(h, "GET", "/api/v1/projects", "u-1")
	if w.Code != 429 || w.Header().Get("Retry-After") != "60" || w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("over the limit = %d %v %s", w.Code, w.Header(), w.Body)
	}
	if w := do(h, "GET", "/api/v1/projects", "u-2"); w.Code != 204 {
		t.Fatalf("another user = %d", w.Code)
	}
	if w := do(h, "POST", "/api/v1/issues/K-1/attachments", "u-3"); w.Code != 204 {
		t.Fatalf("first upload = %d", w.Code)
	}
	if w := do(h, "POST", "/api/v1/issues/K-1/attachments", "u-3"); w.Code != 429 {
		t.Fatalf("second upload = %d", w.Code)
	}
	if w := do(h, "GET", "/api/v1/projects", ""); w.Code != 204 {
		t.Fatalf("anonymous requests are left to their own limits: %d", w.Code)
	}
	// A broken limiter fails open: the API stays up.
	open := ratelimit.Middleware(failing{}, ratelimit.Rules{API: guard.Rule{Limit: 1, Window: time.Minute}}, log)(ok)
	if w := do(open, "GET", "/api/v1/projects", "u-1"); w.Code != 204 {
		t.Fatalf("fail open = %d", w.Code)
	}
	// A zero rule turns limiting off.
	off := ratelimit.Middleware(lim, ratelimit.Rules{}, log)(ok)
	for range 5 {
		if w := do(off, "GET", "/api/v1/projects", "u-9"); w.Code != 204 {
			t.Fatalf("disabled = %d", w.Code)
		}
	}
}
