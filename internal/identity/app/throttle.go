package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// LoginAttempts per LoginWindow are allowed for one email+IP pair.
	LoginAttempts = 10
	LoginWindow   = 15 * time.Minute
)

// LoginLimiter counts login attempts per key. Production uses guard/ratelimit on
// Redis (shared by all replicas); the default is an in-process sliding window.
type LoginLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

// TooManyAttemptsError is returned while an email+IP pair is over its limit.
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many login attempts, retry in %s", e.RetryAfter.Round(time.Second))
}

func throttleKey(email, ip string) string {
	return "login:" + strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}

// localLimiter is a per-process sliding-window log (single-instance deployments only).
type localLimiter struct {
	mu    sync.Mutex
	clock Clock
	hits  map[string][]time.Time
}

func newLocalLimiter(c Clock) *localLimiter {
	return &localLimiter{clock: c, hits: map[string][]time.Time{}}
}

func (l *localLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < LoginWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= LoginAttempts {
		l.hits[key] = recent
		return false, LoginWindow - now.Sub(recent[0]), nil
	}
	l.hits[key] = append(recent, now)
	if len(l.hits) > 10_000 {
		l.gc(now)
	}
	return true, 0, nil
}

// gc bounds memory by dropping keys whose window has fully expired.
func (l *localLimiter) gc(now time.Time) {
	for k, ts := range l.hits {
		if len(ts) == 0 || now.Sub(ts[len(ts)-1]) >= LoginWindow {
			delete(l.hits, k)
		}
	}
}
