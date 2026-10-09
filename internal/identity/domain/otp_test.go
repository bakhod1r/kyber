package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var t0 = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

func delivered(t *testing.T) *domain.OTPChallenge {
	t.Helper()
	c := domain.NewOTPChallenge("c-1", "nonce-1", t0)
	if err := c.Deliver("42", "Bobur", "123456", t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOTPHappyPath(t *testing.T) {
	c := delivered(t)
	if c.ID != "c-1" || c.Nonce != "nonce-1" || !c.ExpiresAt.Equal(t0.Add(domain.OTPTTL)) || c.TelegramID != "42" || c.Name != "Bobur" {
		t.Fatalf("challenge = %+v", c)
	}
	if len(c.CodeHash) == 0 || string(c.CodeHash) == "123456" {
		t.Fatal("the code must be stored hashed")
	}
	if err := c.Verify("123456", t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.Verify("123456", t0.Add(2*time.Minute)); !errors.Is(err, domain.ErrOTPUsed) {
		t.Fatalf("replay err = %v", err)
	}
}

func TestOTPRejects(t *testing.T) {
	late := t0.Add(domain.OTPTTL + time.Second)
	fresh := func() *domain.OTPChallenge { return domain.NewOTPChallenge("c", "n", t0) }

	if err := fresh().Verify("123456", t0); !errors.Is(err, domain.ErrOTPNotDelivered) {
		t.Fatalf("undelivered err = %v", err)
	}
	if err := fresh().Deliver("42", "B", "123456", late); !errors.Is(err, domain.ErrOTPExpired) {
		t.Fatalf("late deliver err = %v", err)
	}
	if err := delivered(t).Deliver("43", "Other", "000000", t0); !errors.Is(err, domain.ErrOTPAlreadyDelivered) {
		t.Fatalf("second deliver err = %v (a second Telegram user must not hijack the challenge)", err)
	}
	if err := delivered(t).Verify("123456", late); !errors.Is(err, domain.ErrOTPExpired) {
		t.Fatalf("expired err = %v", err)
	}
	c := delivered(t)
	for i := 1; i <= domain.OTPMaxAttempts; i++ {
		if err := c.Verify("000000", t0); !errors.Is(err, domain.ErrOTPWrongCode) {
			t.Fatalf("attempt %d err = %v", i, err)
		}
	}
	if err := c.Verify("123456", t0); !errors.Is(err, domain.ErrOTPTooManyAttempts) || c.Attempts != domain.OTPMaxAttempts {
		t.Fatalf("locked err = %v attempts=%d (even the right code fails after too many attempts)", err, c.Attempts)
	}
}

func TestNewOTPCode(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code := domain.NewOTPCode()
		if len(code) != 6 || code[0] < '0' || code[5] > '9' {
			t.Fatalf("code = %q", code)
		}
		seen[code] = true
	}
	if len(seen) < 190 {
		t.Fatalf("codes repeat too often: %d unique of 200", len(seen))
	}
}
