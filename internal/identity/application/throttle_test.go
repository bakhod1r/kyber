package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
)

func TestLoginLimitedToTenAttemptsPerWindow(t *testing.T) {
	ctx := context.Background()
	s, c, _ := setup()
	_, _ = s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	for i := range 9 {
		if _, err := s.Login(ctx, "ali@x.uz", "wrong password", "10.0.0.1"); !errors.Is(err, application.ErrInvalidCredentials) {
			t.Fatalf("attempt %d err = %v", i+1, err)
		}
	}
	if _, err := s.Login(ctx, "ALI@x.uz", "long enough pw", "10.0.0.1"); err != nil {
		t.Fatalf("10th attempt (correct) err = %v", err)
	}
	_, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.1")
	var tm *application.TooManyAttemptsError
	if !errors.As(err, &tm) || tm.RetryAfter <= 0 || tm.RetryAfter > 15*time.Minute {
		t.Fatalf("11th attempt err = %v, want TooManyAttemptsError", err)
	}
	if _, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.2"); err != nil {
		t.Fatalf("other IP: %v", err)
	}
	c.now = c.now.Add(15*time.Minute + time.Second)
	if _, err := s.Login(ctx, "ali@x.uz", "long enough pw", "10.0.0.1"); err != nil {
		t.Fatalf("after window: %v", err)
	}
}

type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return false, 0, errors.New("redis down")
}

func TestLimiterOutageFailsClosed(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup(application.WithLoginLimiter(failingLimiter{}))
	_, _ = s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})
	if _, err := s.Login(ctx, "ali@x.uz", "long enough pw", "ip"); err == nil || errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("limiter outage must reject logins with an internal error, got %v", err)
	}
}
