package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
)

func setupExternal() (*application.Service, *memory.Repository) {
	repo := memory.NewRepository()
	n := 0
	ids := func() string { n++; return []string{"", "u-1", "u-2", "u-3", "u-4"}[n] }
	c := &clock{}
	return application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithExternalIdentities(repo)), repo
}

func TestLoginExternal(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	google := application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-123", Email: "Ali@Gmail.com", EmailVerified: true, Name: "Ali"}

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
	if _, err := s.Login(ctx, "ali@gmail.com", "", "1.1.1.1"); !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("password login err = %v", err)
	}
}

func TestLoginExternalLinksVerifiedEmailOnly(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	existing, _ := s.Signup(ctx, application.Signup{Email: "dev@x.uz", Name: "Dev", Password: "long enough pw"})

	// An unverified email must not take over an existing account.
	_, _, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "evil", Email: "dev@x.uz", Name: "Evil"})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("unverified err = %v", err)
	}
	// A verified email links to the existing account; the password keeps working.
	_, u, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-dev", Email: "DEV@x.uz", EmailVerified: true, Name: "Dev G"})
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
	tg := application.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "42", Name: "Bobur"}
	_, u, err := s.LoginExternal(ctx, tg)
	if err != nil || u.Email() != "telegram-42@users.kyber.invalid" || u.Name() != "Bobur" {
		t.Fatalf("telegram = %+v %v", u, err)
	}
	if _, u2, err := s.LoginExternal(ctx, tg); err != nil || u2.ID() != u.ID() {
		t.Fatalf("telegram again = %+v %v", u2, err)
	}
	// Validation: provider and subject are required; a blank name falls back to a default.
	for _, bad := range []application.ExternalProfile{{Subject: "1"}, {Provider: domain.ProviderGoogle}, {Provider: "myspace", Subject: "1"}} {
		if _, _, err := s.LoginExternal(ctx, bad); !errors.Is(err, application.ErrInvalidProfile) {
			t.Errorf("%+v err = %v", bad, err)
		}
	}
	if _, u, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "7"}); err != nil || u.Name() != "Telegram user 7" {
		t.Fatalf("nameless = %+v %v", u, err)
	}
	// A verified email that is malformed is treated as missing.
	if _, u, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "x", Email: "nope", EmailVerified: true, Name: "N"}); err != nil || u.Email() != "google-x@users.kyber.invalid" {
		t.Fatalf("bad email = %+v %v", u, err)
	}
}

func TestLoginExternalNotConfigured(t *testing.T) {
	s, _, _ := setup()
	if _, _, err := s.LoginExternal(context.Background(), application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "1"}); !errors.Is(err, application.ErrExternalDisabled) {
		t.Fatalf("err = %v", err)
	}
}

// failing wraps the memory repository and fails one operation.
type failing struct {
	*memory.Repository
	op string
}

var errBoom = errors.New("boom")

func (f *failing) LinkedUser(ctx context.Context, p domain.Provider, s string) (domain.UserID, error) {
	if f.op == "linked" {
		return "", errBoom
	}
	return f.Repository.LinkedUser(ctx, p, s)
}

func (f *failing) Link(ctx context.Context, id domain.ExternalIdentity) error {
	if f.op == "link" {
		return errBoom
	}
	return f.Repository.Link(ctx, id)
}

func (f *failing) ByEmail(ctx context.Context, e domain.Email) (*domain.User, error) {
	if f.op == "byemail" {
		return nil, errBoom
	}
	return f.Repository.ByEmail(ctx, e)
}

func (f *failing) Create(ctx context.Context, u *domain.User) error {
	if f.op == "create" {
		return errBoom
	}
	return f.Repository.Create(ctx, u)
}

func (f *failing) CreateSession(ctx context.Context, s domain.Session) error {
	if f.op == "session" {
		return errBoom
	}
	return f.Repository.CreateSession(ctx, s)
}

func TestLoginExternalStorageErrors(t *testing.T) {
	for _, op := range []string{"linked", "link", "byemail", "create", "session"} {
		f := &failing{memory.NewRepository(), op}
		s := application.NewService(f, f, &plainHasher{}, &clock{}, func() string { return "u-1" }, application.WithExternalIdentities(f))
		_, _, err := s.LoginExternal(context.Background(), application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "1", Email: "a@b.uz", EmailVerified: true, Name: "A"})
		if !errors.Is(err, errBoom) {
			t.Errorf("%s: err = %v", op, err)
		}
	}
	f := &failing{memory.NewRepository(), "byemail"}
	s := application.NewService(f, f, &plainHasher{}, &clock{}, func() string { return "u-1" })
	if _, err := s.Login(context.Background(), "a@b.uz", "pw", "1.1.1.1"); !errors.Is(err, errBoom) {
		t.Errorf("login byemail err = %v", err)
	}
}

func TestLoginExternalBlankName(t *testing.T) {
	s, _ := setupExternal()
	_, u, err := s.LoginExternal(context.Background(), application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "9", Email: "x@gmail.com", EmailVerified: true, Name: "   "})
	if err != nil || u.Name() != "Google user 9" {
		t.Fatalf("blank name = %+v %v", u, err)
	}
}

// Regression (review): if linking failed after the account was created, the next login must
// recover instead of reporting "email taken" forever for the provider's synthetic address.
func TestLoginExternalRecoversFromFailedLink(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRepository()
	f := &failing{repo, "link"}
	s := application.NewService(f, f, &plainHasher{}, &clock{}, func() string { return "u-1" }, application.WithExternalIdentities(f))
	tg := application.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "42", Name: "Bobur"}
	if _, _, err := s.LoginExternal(ctx, tg); !errors.Is(err, errBoom) {
		t.Fatalf("first err = %v", err)
	}
	f.op = "" // the database is back
	_, u, err := s.LoginExternal(ctx, tg)
	if err != nil || u.ID() != "u-1" {
		t.Fatalf("second login = %+v %v", u, err)
	}
	if id, err := repo.LinkedUser(ctx, domain.ProviderTelegram, "42"); err != nil || id != "u-1" {
		t.Fatalf("link repaired = %s %v", id, err)
	}
}

// A concurrent first login that loses the race to create the account signs into it.
func TestLoginExternalCreateRace(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRepository()
	r := &racing{Repository: repo}
	s := application.NewService(r, r, &plainHasher{}, &clock{}, func() string { return "u-2" }, application.WithExternalIdentities(r))
	_, u, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGoogle, Subject: "g-1", Email: "a@gmail.com", EmailVerified: true, Name: "A"})
	if err != nil || u.ID() != "u-winner" {
		t.Fatalf("race = %+v %v", u, err)
	}
}

// racing makes the first ByEmail miss and Create collide with a winner that just created and linked the account.
type racing struct {
	*memory.Repository
	done bool
}

func (r *racing) Create(ctx context.Context, u *domain.User) error {
	if !r.done {
		r.done = true
		w, _ := domain.NewUser("u-winner", u.Email(), "A", "")
		_ = r.Repository.Create(ctx, w)
		_ = r.Repository.Link(ctx, domain.ExternalIdentity{Provider: domain.ProviderGoogle, Subject: "g-1", UserID: "u-winner"})
	}
	return r.Repository.Create(ctx, u)
}

// The synthetic domain is reserved: nobody can sign up with it to capture a provider account.
func TestSignupRejectsReservedDomain(t *testing.T) {
	s, _ := setupExternal()
	_, err := s.Signup(context.Background(), application.Signup{Email: "Telegram-42@users.kyber.invalid", Name: "Evil", Password: "long enough pw"})
	if !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("err = %v", err)
	}
}

// F6: generic OpenID Connect and GitHub identities link like Google's.
func TestLoginExternalSSOProviders(t *testing.T) {
	ctx := context.Background()
	s, _ := setupExternal()
	for _, tc := range []struct {
		p    domain.Provider
		name string
	}{{domain.ProviderOIDC, "SSO user 5"}, {domain.ProviderGitHub, "GitHub user 5"}} {
		_, u, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: tc.p, Subject: "5"})
		if err != nil || u.Name() != tc.name || string(u.Email()) != string(tc.p)+"-5@users.kyber.invalid" {
			t.Fatalf("%s = %+v %v", tc.p, u, err)
		}
	}
	_, a, _ := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderOIDC, Subject: "o", Email: "same@x.uz", EmailVerified: true, Name: "S"})
	_, b, err := s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderGitHub, Subject: "77", Email: "same@x.uz", EmailVerified: true, Name: "S"})
	if err != nil || a.ID() != b.ID() {
		t.Fatalf("verified email must link: %v %v %v", a, b, err)
	}
}

type groupSync struct {
	calls []string
	err   error
}

func (g *groupSync) Sync(_ context.Context, user string, groups []string) error {
	g.calls = append(g.calls, user+":"+strings.Join(groups, ","))
	return g.err
}

// S106: SSO logins hand the user's groups to group sync; a sync failure never blocks the login.
func TestLoginExternalSyncsGroups(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRepository()
	n := 0
	ids := func() string { n++; return []string{"", "u-1", "u-2"}[n] }
	sync := &groupSync{err: errors.New("project gone")}
	s := application.NewService(repo, repo, &plainHasher{}, &clock{}, ids, application.WithExternalIdentities(repo), application.WithGroupSync(sync))
	p := application.ExternalProfile{Provider: domain.ProviderOIDC, Subject: "o-1", Email: "ann@corp.example", EmailVerified: true, Name: "Ann", Groups: []string{"eng", "leads"}}
	token, u, err := s.LoginExternal(ctx, p)
	if err != nil || token == "" {
		t.Fatalf("login = %v", err)
	}
	if len(sync.calls) != 1 || sync.calls[0] != string(u.ID())+":eng,leads" {
		t.Fatalf("sync = %v", sync.calls)
	}
	// Telegram and Google carry no groups: nothing is synced.
	_, _, _ = s.LoginExternal(ctx, application.ExternalProfile{Provider: domain.ProviderTelegram, Subject: "t-1", Name: "T"})
	if len(sync.calls) != 1 {
		t.Fatalf("synced a provider without groups: %v", sync.calls)
	}
}
