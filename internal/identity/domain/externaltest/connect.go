package externaltest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// RunConnect is the contract every ConnectTokens adapter must pass; user must exist.
func RunConnect(t *testing.T, s domain.ConnectTokens, user domain.UserID) {
	ctx := context.Background()
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	raw, tok := domain.NewConnectToken(user, now)
	if err := s.CreateConnect(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ConsumeConnect(ctx, domain.HashConnectToken(raw), now.Add(time.Minute)); err != nil || got != user {
		t.Fatalf("consume = %q, %v", got, err)
	}
	if _, err := s.ConsumeConnect(ctx, domain.HashConnectToken(raw), now.Add(time.Minute)); !errors.Is(err, domain.ErrConnectTokenInvalid) {
		t.Fatalf("second use err = %v", err)
	}
	if _, err := s.ConsumeConnect(ctx, domain.HashConnectToken("c_unknown"), now); !errors.Is(err, domain.ErrConnectTokenInvalid) {
		t.Fatalf("unknown err = %v", err)
	}
	raw, tok = domain.NewConnectToken(user, now)
	_ = s.CreateConnect(ctx, tok)
	if _, err := s.ConsumeConnect(ctx, domain.HashConnectToken(raw), now.Add(domain.ConnectTTL+time.Second)); !errors.Is(err, domain.ErrConnectTokenInvalid) {
		t.Fatalf("expired err = %v", err)
	}
	// Two bots updates racing for the same token: exactly one wins.
	raw, tok = domain.NewConnectToken(user, now)
	_ = s.CreateConnect(ctx, tok)
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ConsumeConnect(ctx, domain.HashConnectToken(raw), now); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("token used %d times", wins)
	}
}
