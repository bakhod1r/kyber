// Package ratelimit limits each signed-in user's API requests (sliding window), in process or,
// with Redis, across replicas through guard's limiter. Uploads have their own, tighter rule.
package ratelimit

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	guard "github.com/bakhod1r/guard/ratelimit"

	"github.com/bakhod1r/errorx"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/httpx"
)

// Memory is a per-process sliding-window log limiter (one replica, dev and tests).
type Memory struct {
	mu   sync.Mutex
	now  func() time.Time
	hits map[string][]time.Time
	seen int
}

func NewMemory(now func() time.Time) *Memory {
	return &Memory{now: now, hits: map[string][]time.Time{}}
}

func (m *Memory) Allow(_ context.Context, key string, rule guard.Rule) (guard.Result, error) {
	if err := rule.Valid(); err != nil {
		return guard.Result{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	keep := func(ts []time.Time) []time.Time {
		n := 0
		for n < len(ts) && !ts[n].After(now.Add(-rule.Window)) {
			n++
		}
		return ts[n:]
	}
	if m.seen++; m.seen%1024 == 0 { // drop idle keys now and then so memory stays bounded
		for k, ts := range m.hits {
			if len(keep(ts)) == 0 {
				delete(m.hits, k)
			}
		}
	}
	ts := keep(m.hits[key])
	res := guard.Result{Limit: rule.Limit}
	if len(ts) >= rule.Limit {
		res.RetryAfter = ts[0].Add(rule.Window).Sub(now)
		res.ResetAfter = res.RetryAfter
		m.hits[key] = ts
		return res, nil
	}
	ts = append(ts, now)
	m.hits[key] = ts
	res.Allowed, res.Remaining, res.ResetAfter = true, rule.Limit-len(ts), ts[0].Add(rule.Window).Sub(now)
	return res, nil
}

// Rules are the limits per user; a zero rule turns that limit off.
type Rules struct {
	API    guard.Rule // every authenticated API request
	Upload guard.Rule // attachment and file uploads (POST …/attachments, …/files)
}

// Middleware enforces rules per signed-in user (auth.Actor); anonymous requests are left to the
// limits of their own endpoints (login). A failing limiter fails open, so Redis trouble never
// takes the API down.
func Middleware(l guard.Limiter, rules Rules, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.Actor(r.Context())
			rule, bucket := rules.API, "api:"
			if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/attachments") || strings.HasSuffix(r.URL.Path, "/files")) && rules.Upload.Limit > 0 {
				rule, bucket = rules.Upload, "upload:"
			}
			if user == "" || rule.Limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			res, err := l.Allow(r.Context(), bucket+user, rule)
			if err != nil {
				log.WarnContext(r.Context(), "rate limiter unavailable", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
			if !res.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(res.RetryAfter.Seconds()))))
				httpx.Problem(w, r, errorx.New(httpx.CodeRateLimited, "rate limited"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
