package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// plainHasher is a fast fake for tests; production uses argon2id.
type plainHasher struct{ verifies int }

func (plainHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (h *plainHasher) Verify(hash, p string) bool { h.verifies++; return hash == "h:"+p }

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func setup() (*app.Service, *clock, *plainHasher) {
	n := 0
	ids := func() string { n++; return fmt.Sprintf("u-%d", n) }
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	h := &plainHasher{}
	repo := memory.NewRepository()
	return app.NewService(repo, repo, h, c, ids), c, h
}

func TestSignup(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	u, err := s.Signup(ctx, app.Signup{Email: " Ali@X.uz ", Name: "Ali", Password: "long enough pw"})
	if err != nil || u.Email().String() != "ali@x.uz" || u.PasswordHash() != "h:long enough pw" {
		t.Fatalf("u = %+v, %v", u, err)
	}
	cases := []struct {
		cmd  app.Signup
		want error
	}{
		{app.Signup{Email: "ALI@x.uz", Name: "Dup", Password: "long enough pw"}, domain.ErrEmailTaken},
		{app.Signup{Email: "bad", Name: "B", Password: "long enough pw"}, domain.ErrInvalidEmail},
		{app.Signup{Email: "b@x.uz", Name: "B", Password: "short"}, domain.ErrWeakPassword},
		{app.Signup{Email: "b@x.uz", Name: " ", Password: "long enough pw"}, domain.ErrEmptyName},
	}
	for _, c := range cases {
		if _, err := s.Signup(ctx, c.cmd); !errors.Is(err, c.want) {
			t.Errorf("Signup(%+v) err = %v, want %v", c.cmd, err, c.want)
		}
	}
}

func TestLoginAuthenticateLogout(t *testing.T) {
	ctx := context.Background()
	s, c, _ := setup()
	_, _ = s.Signup(ctx, app.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	token, err := s.Login(ctx, "ALI@x.uz", "long enough pw")
	if err != nil || len(token) < 40 {
		t.Fatalf("token = %q, %v", token, err)
	}
	u, err := s.Authenticate(ctx, token)
	if err != nil || u.Name() != "Ali" {
		t.Fatalf("Authenticate = %+v, %v", u, err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("after logout err = %v", err)
	}

	// Expiry after 30 days.
	token, _ = s.Login(ctx, "ali@x.uz", "long enough pw")
	c.now = c.now.Add(30*24*time.Hour + time.Second)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("expired err = %v", err)
	}
	if _, err := s.Authenticate(ctx, "garbage"); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("garbage err = %v", err)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	ctx := context.Background()
	s, _, h := setup()
	_, _ = s.Signup(ctx, app.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	_, errWrongPw := s.Login(ctx, "ali@x.uz", "wrong password")
	before := h.verifies
	_, errNoUser := s.Login(ctx, "nobody@x.uz", "wrong password")
	if !errors.Is(errWrongPw, app.ErrInvalidCredentials) || !errors.Is(errNoUser, app.ErrInvalidCredentials) {
		t.Fatalf("errs = %v / %v", errWrongPw, errNoUser)
	}
	if errWrongPw.Error() != errNoUser.Error() {
		t.Fatal("messages must be identical (no user enumeration)")
	}
	if h.verifies != before+1 {
		t.Fatal("unknown user must still run a hash verification (timing)")
	}
}
