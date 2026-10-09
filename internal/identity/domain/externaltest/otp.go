package externaltest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// RunOTP is the contract every OTPChallenges adapter must pass.
func RunOTP(t *testing.T, s domain.OTPChallenges) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	const id, nonce = "50000000-0000-4000-8000-000000000001", "nonce-abc"
	if err := s.CreateOTP(ctx, domain.NewOTPChallenge(id, nonce, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OTPByID(ctx, "50000000-0000-4000-8000-000000000009"); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("missing id err = %v", err)
	}
	if _, err := s.OTPByNonce(ctx, "nope"); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("missing nonce err = %v", err)
	}
	if err := s.UpdateOTP(ctx, "50000000-0000-4000-8000-000000000009", func(*domain.OTPChallenge) error { return nil }); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("update missing err = %v", err)
	}
	if err := s.UpdateOTP(ctx, id, func(c *domain.OTPChallenge) error { return c.Deliver("42", "Bobur", "123456", now) }); err != nil {
		t.Fatal(err)
	}
	// A failing fn still persists its changes (wrong codes are counted) and returns the error.
	if err := s.UpdateOTP(ctx, id, func(c *domain.OTPChallenge) error { return c.Verify("000000", now) }); !errors.Is(err, domain.ErrOTPWrongCode) {
		t.Fatalf("wrong code err = %v", err)
	}
	got, err := s.OTPByNonce(ctx, nonce)
	if err != nil || got.ID != id || got.TelegramID != "42" || got.Name != "Bobur" || got.Attempts != 1 || len(got.CodeHash) == 0 ||
		!got.ExpiresAt.Equal(now.Add(domain.OTPTTL)) || got.Used {
		t.Fatalf("stored = %+v, %v", got, err)
	}
	// Concurrent guesses are serialised: every attempt is counted, none is lost.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.UpdateOTP(ctx, id, func(c *domain.OTPChallenge) error { return c.Verify("999999", now) })
		}()
	}
	wg.Wait()
	if got, _ := s.OTPByID(ctx, id); got.Attempts != domain.OTPMaxAttempts {
		t.Fatalf("attempts after concurrent guesses = %d, want %d", got.Attempts, domain.OTPMaxAttempts)
	}
	// Expired challenges are purged; live ones stay.
	const live = "50000000-0000-4000-8000-000000000002"
	_ = s.CreateOTP(ctx, domain.NewOTPChallenge(live, "nonce-live", now.Add(time.Hour)))
	if n, err := s.DeleteExpiredOTPs(ctx, now.Add(domain.OTPTTL+time.Second)); err != nil || n != 1 {
		t.Fatalf("purged = %d, %v", n, err)
	}
	if _, err := s.OTPByID(ctx, id); !errors.Is(err, domain.ErrOTPNotFound) {
		t.Fatalf("expired still there: %v", err)
	}
	if _, err := s.OTPByID(ctx, live); err != nil {
		t.Fatalf("live challenge purged: %v", err)
	}
}
