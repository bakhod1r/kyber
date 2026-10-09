package app

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	maxFailures    = 5
	throttleWindow = 15 * time.Minute
)

// TooManyAttemptsError is returned while an email+IP pair is locked out.
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many login attempts, retry in %s", e.RetryAfter.Round(time.Second))
}

// throttle counts failed logins per email+IP in a fixed window.
// It is per-process: multi-instance deployments need a shared store (tracked in docs/qa).
type throttle struct {
	mu      sync.Mutex
	clock   Clock
	entries map[string]attempts
}

type attempts struct {
	failures int
	since    time.Time
}

func newThrottle(c Clock) *throttle { return &throttle{clock: c, entries: map[string]attempts{}} }

func throttleKey(email, ip string) string {
	return strings.ToLower(strings.TrimSpace(email)) + "|" + ip
}

func (t *throttle) check(key string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, ok := t.entries[key]
	if !ok {
		return nil
	}
	elapsed := t.clock.Now().Sub(a.since)
	if elapsed >= throttleWindow {
		delete(t.entries, key)
		return nil
	}
	if a.failures >= maxFailures {
		return &TooManyAttemptsError{RetryAfter: throttleWindow - elapsed}
	}
	return nil
}

func (t *throttle) fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, ok := t.entries[key]
	if !ok || t.clock.Now().Sub(a.since) >= throttleWindow {
		a = attempts{since: t.clock.Now()}
	}
	a.failures++
	t.entries[key] = a
	t.gc()
}

func (t *throttle) reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// gc bounds memory by dropping expired windows once the map grows large.
func (t *throttle) gc() {
	if len(t.entries) < 10_000 {
		return
	}
	now := t.clock.Now()
	for k, a := range t.entries {
		if now.Sub(a.since) >= throttleWindow {
			delete(t.entries, k)
		}
	}
}
