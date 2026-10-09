package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/app"
)

func TestThrottleBlocksAfterFiveFailures(t *testing.T) {
	ctx := context.Background()
	s, c, _ := setup()
	_, _ = s.Signup(ctx, app.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	for i := range 5 {
		if _, err := s.Login(ctx, "ali@x.uz", "wrong password", "10.0.0.1"); !errors.Is(err, app.ErrInvalidCredentials) {
			t.Fatalf("attempt %d err = %v", i+1, err)
		}
	}
	_, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.1")
	var tm *app.TooManyAttemptsError
	if !errors.As(err, &tm) || tm.RetryAfter <= 0 || tm.RetryAfter > 15*time.Minute {
		t.Fatalf("6th attempt err = %v, want TooManyAttemptsError", err)
	}
	// Other IPs and other emails are unaffected.
	if _, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.2"); err != nil {
		t.Fatalf("other ip: %v", err)
	}
	// The window expires.
	c.now = c.now.Add(15*time.Minute + time.Second)
	if _, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.1"); err != nil {
		t.Fatalf("after window: %v", err)
	}
}

func TestThrottleResetsOnSuccess(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	_, _ = s.Signup(ctx, app.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	for range 4 {
		_, _ = s.Login(ctx, "ali@x.uz", "wrong password", "ip")
	}
	if _, err := s.Login(ctx, "ALI@x.uz", "long enough pw", "ip"); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if _, err := s.Login(ctx, "ali@x.uz", "wrong password", "ip"); !errors.Is(err, app.ErrInvalidCredentials) {
			t.Fatalf("attempt %d after reset err = %v", i+1, err)
		}
	}
}
