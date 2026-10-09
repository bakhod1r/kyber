package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/adapter/memory"
	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func setupExternal() (*app.Service, *memory.Repository) {
	repo := memory.NewRepository()
	n := 0
	ids := func() string { n++; return []string{"", "u-1", "u-2", "u-3", "u-4"}[n] }
	c := &clock{}
	return app.NewService(repo, repo, &plainHasher{}, c, ids, app.WithExternalIdentities(repo)), repo
}

func TestLoginExternal(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	google := app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-123", Email: "Ali@Gmail.com", EmailVerified: true, Name: "Ali"}

	// AC1 first login creates an account without a password.
	token, u, err := s.LoginExternal(ctx, google)
	if err != nil || token == "" || u.Email() != "ali@gmail.com" || u.Name() != "Ali" || u.PasswordHash() != "" {
		t.Fatalf("first login = %q %+v %v", token, u, err)
	}
	if me, err := s.Authenticate(ctx, token); err != nil || me.ID() != u.ID() {
		t.Fatalf("session = %+v %v", me, err)
	}
	// AC2 next login finds the linked account even if the email changed at Google.
	google.Email = "renamed@gmail.com"
	_, again, err := s.LoginExternal(ctx, google)
	if err != nil || again.ID() != u.ID() {
		t.Fatalf("second login = %+v %v", again, err)
	}
	// AC3 a social-only account cannot log in with a password (no empty-hash bypass).
	if _, err := s.Login(ctx, "ali@gmail.com", "", "1.1.1.1"); !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("password login err = %v", err)
	}
}

func TestLoginExternalLinksVerifiedEmailOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	existing, _ := s.Signup(ctx, app.Signup{Email: "dev@x.uz", Name: "Dev", Password: "long enough pw"})

	// An unverified email must not take over an existing account.
	_, _, err := s.LoginExternal(ctx, app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "evil", Email: "dev@x.uz", Name: "Evil"})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("unverified err = %v", err)
	}
	// A verified email links to the existing account; the password keeps working.
	_, u, err := s.LoginExternal(ctx, app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-dev", Email: "DEV@x.uz", EmailVerified: true, Name: "Dev G"})
	if err != nil || u.ID() != existing.ID() {
		t.Fatalf("link = %+v %v", u, err)
	}
	if _, err := s.Login(ctx, "dev@x.uz", "long enough pw", "1.1.1.1"); err != nil {
		t.Fatalf("password login after link: %v", err)
	}
}

func TestLoginExternalTelegram(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	tg := app.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "42", Name: "Bobur"}
	_, u, err := s.LoginExternal(ctx, tg)
	if err != nil || u.Email() != "telegram-42@users.kyber.invalid" || u.Name() != "Bobur" {
		t.Fatalf("telegram = %+v %v", u, err)
	}
	if _, u2, err := s.LoginExternal(ctx, tg); err != nil || u2.ID() != u.ID() {
		t.Fatalf("telegram again = %+v %v", u2, err)
	}
	// Validation: provider and subject are required; a blank name falls back to a default.
	for _, bad := range []app.ExternalProfile{{Subject: "1"}, {Provider: domain.ProviderGoogle}, {Provider: "myspace", Subject: "1"}} {
		if _, _, err := s.LoginExternal(ctx, bad); !errors.Is(err, app.ErrInvalidProfile) {
			t.Errorf("%+v err = %v", bad, err)
		}
	}
	if _, u, err := s.LoginExternal(ctx, app.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "7"}); err != nil || u.Name() != "Telegram user 7" {
		t.Fatalf("nameless = %+v %v", u, err)
	}
	// A verified email that is malformed is treated as missing.
	if _, u, err := s.LoginExternal(ctx, app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "x", Email: "nope", EmailVerified: true, Name: "N"}); err != nil || u.Email() != "google-x@users.kyber.invalid" {
		t.Fatalf("bad email = %+v %v", u, err)
	}
}

func TestLoginExternalNotConfigured(t *testing.T) {
	s, _, _ := setup()
	if _, _, err := s.LoginExternal(context.Background(), app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "1"}); !errors.Is(err, app.ErrExternalDisabled) {
		t.Fatalf("err = %v", err)
	}
}

// failing wraps the memory repository and fails one operation.
type failing struct {
	*memory.Repository
	op string
}

var errBoom = errors.New("boom")

func (f failing) LinkedUser(ctx context.Context, p domain.Provider, s string) (domain.UserID, error) {
	if f.op == "linked" {
		return "", errBoom
	}
	return f.Repository.LinkedUser(ctx, p, s)
}

func (f failing) Link(ctx context.Context, id domain.ExternalIdentity) error {
	if f.op == "link" {
		return errBoom
	}
	return f.Repository.Link(ctx, id)
}

func (f failing) ByEmail(ctx context.Context, e domain.Email) (*domain.User, error) {
	if f.op == "byemail" {
		return nil, errBoom
	}
	return f.Repository.ByEmail(ctx, e)
}

func (f failing) Create(ctx context.Context, u *domain.User) error {
	if f.op == "create" {
		return errBoom
	}
	return f.Repository.Create(ctx, u)
}

func (f failing) CreateSession(ctx context.Context, s domain.Session) error {
	if f.op == "session" {
		return errBoom
	}
	return f.Repository.CreateSession(ctx, s)
}

func TestLoginExternalStorageErrors(t *testing.T) {
	for _, op := range []string{"linked", "link", "byemail", "create", "session"} {
		f := failing{memory.NewRepository(), op}
		s := app.NewService(f, f, &plainHasher{}, &clock{}, func() string { return "u-1" }, app.WithExternalIdentities(f))
		_, _, err := s.LoginExternal(context.Background(), app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "1", Email: "a@b.uz", EmailVerified: true, Name: "A"})
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: err = %v", op, err)
		}
	}
	f := failing{memory.NewRepository(), "byemail"}
	s := app.NewService(f, f, &plainHasher{}, &clock{}, func() string { return "u-1" })
	if _, err := s.Login(context.Background(), "a@b.uz", "pw", "1.1.1.1"); !errors.Is(err, errBoom) {
		t.Errorf("login byemail err = %v", err)
	}
}

func TestLoginExternalBlankName(t *testing.T) {
	s, _ := setupExternal()
	_, u, err := s.LoginExternal(context.Background(), app.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "9", Email: "x@gmail.com", EmailVerified: true, Name: "   "})
	if err != nil || u.Name() != "Google user 9" {
		t.Fatalf("blank name = %+v %v", u, err)
	}
}
