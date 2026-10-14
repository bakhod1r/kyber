package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/memory"
)

type failingTokens struct {
	*memory.Repository
	create, find, touch error
}

func (f failingTokens) CreateToken(ctx context.Context, t *domain.PersonalToken) error {
	if f.create != nil {
		return f.create
	}
	return f.Repository.CreateToken(ctx, t)
}

func (f failingTokens) TokenByHash(ctx context.Context, h []byte) (*domain.PersonalToken, error) {
	if f.find != nil {
		return nil, f.find
	}
	return f.Repository.TokenByHash(ctx, h)
}

func (f failingTokens) TouchToken(ctx context.Context, id domain.TokenID, at time.Time) error {
	if f.touch != nil {
		return f.touch
	}
	return f.Repository.TouchToken(ctx, id, at)
}

func tokenService(tokens func(*memory.Repository) domain.PersonalTokens) (*application.Service, *memory.Repository, *clock) {
	n := 0
	ids := func() string { n++; return "id-" + string(rune('0'+n)) }
	c := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	repo := memory.NewRepository()
	return application.NewService(repo, repo, &plainHasher{}, c, ids, application.WithPersonalTokens(tokens(repo))), repo, c
}

func plain(r *memory.Repository) domain.PersonalTokens { return r }

func TestPersonalTokens(t *testing.T) {
	ctx := context.Background()
	s, _, c := tokenService(plain)
	u, _ := s.Signup(ctx, application.Signup{Email: "ali@x.uz", Name: "Ali", Password: "long enough pw"})

	secret, tok, err := s.CreateToken(ctx, u.ID(), "MCP", 30)
	if err != nil || !strings.HasPrefix(secret, "kpat_") || len(secret) < 40 || tok.ExpiresAt == nil || !application.IsPersonalToken(secret) {
		t.Fatalf("create = %q %+v %v", secret, tok, err)
	}
	if string(tok.Hash) == secret {
		t.Fatal("the secret itself was stored")
	}
	got, err := s.Authenticate(ctx, secret)
	if err != nil || got.ID() != u.ID() {
		t.Fatalf("authenticate = %v, %v", got, err)
	}
	list, _ := s.Tokens(ctx, u.ID())
	if len(list) != 1 || list[0].LastUsedAt == nil {
		t.Fatalf("tokens = %v", list)
	}
	if _, err := s.Authenticate(ctx, "kpat_wrong"); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("wrong secret: %v", err)
	}
	c.now = c.now.AddDate(0, 0, 30)
	if _, err := s.Authenticate(ctx, secret); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("expired token: %v", err)
	}

	forever, never, _ := s.CreateToken(ctx, u.ID(), "CI", 0)
	if never.ExpiresAt != nil {
		t.Fatal("days = 0 must never expire")
	}
	if err := s.RevokeToken(ctx, u.ID(), string(never.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, forever); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("revoked token: %v", err)
	}
	for _, days := range []int{-1, application.MaxTokenDays + 1} {
		if _, _, err := s.CreateToken(ctx, u.ID(), "x", days); !errors.Is(err, domain.ErrInvalidToken) {
			t.Fatalf("days %d: %v", days, err)
		}
	}
	if _, _, err := s.CreateToken(ctx, u.ID(), "", 1); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("empty name: %v", err)
	}
}

func TestPersonalTokenFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("db down")
	s, _, _ := tokenService(func(r *memory.Repository) domain.PersonalTokens { return failingTokens{Repository: r, create: boom} })
	if _, _, err := s.CreateToken(ctx, "u", "x", 1); !errors.Is(err, boom) {
		t.Fatalf("create: %v", err)
	}
	s, _, _ = tokenService(func(r *memory.Repository) domain.PersonalTokens { return failingTokens{Repository: r, find: boom} })
	if _, err := s.Authenticate(ctx, "kpat_x"); !errors.Is(err, boom) {
		t.Fatalf("find: %v", err)
	}
	var s2 *application.Service
	s2, _, _ = tokenService(func(r *memory.Repository) domain.PersonalTokens { return failingTokens{Repository: r, touch: boom} })
	secret, _, _ := s2.CreateToken(ctx, "u", "x", 1)
	if _, err := s2.Authenticate(ctx, secret); !errors.Is(err, boom) {
		t.Fatalf("touch: %v", err)
	}
	// The owner was deleted: the token no longer authenticates.
	s3, _, _ := tokenService(plain)
	orphan, _, _ := s3.CreateToken(ctx, "ghost", "x", 1)
	if _, err := s3.Authenticate(ctx, orphan); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("orphan: %v", err)
	}
	// Without the option, personal tokens never authenticate.
	plainSvc, _, _ := setup()
	if _, err := plainSvc.Authenticate(ctx, "kpat_x"); !errors.Is(err, application.ErrUnauthenticated) {
		t.Fatalf("disabled: %v", err)
	}
}

// S84: a bot is a user without a password (it never signs in) and with a token of its own.
func TestCreateBot(t *testing.T) {
	ctx := context.Background()
	s, _, _ := tokenService(plain)
	bot, secret, err := s.CreateBot(ctx, "CI bot")
	if err != nil || bot.Name() != "CI bot" || !strings.HasSuffix(string(bot.Email()), ".invalid") || bot.PasswordHash() != "" {
		t.Fatalf("bot = %+v %v", bot, err)
	}
	got, err := s.Authenticate(ctx, secret)
	if err != nil || got.ID() != bot.ID() {
		t.Fatalf("token = %v %v", got, err)
	}
	if _, err := s.Login(ctx, string(bot.Email()), "", "1.2.3.4"); err == nil {
		t.Fatal("a bot signed in")
	}
	if _, _, err := s.CreateBot(ctx, " "); !errors.Is(err, domain.ErrEmptyName) {
		t.Fatalf("no name = %v", err)
	}
}
