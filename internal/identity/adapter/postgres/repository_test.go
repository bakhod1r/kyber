package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	r := postgres.NewRepository(dbtest.New(t))
	email, _ := domain.ParseEmail("ali@x.uz")
	u, _ := domain.NewUser("30000000-0000-4000-8000-000000000001", email, "Ali", "hash")

	if err := r.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	dup, _ := domain.NewUser("30000000-0000-4000-8000-000000000002", email, "Dup", "hash")
	if err := r.Create(ctx, dup); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("dup err = %v", err)
	}
	if got, err := r.ByEmail(ctx, email); err != nil || got.ID() != u.ID() || got.PasswordHash() != "hash" {
		t.Fatalf("ByEmail = %+v, %v", got, err)
	}
	if got, err := r.ByID(ctx, u.ID()); err != nil || got.Name() != "Ali" {
		t.Fatalf("ByID = %+v, %v", got, err)
	}
	if _, err := r.ByEmail(ctx, "none@x.uz"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.ByID(ctx, "30000000-0000-4000-8000-000000000009"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("err = %v", err)
	}

	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	s := domain.Session{TokenHash: []byte("0123456789abcdef0123456789abcdef"), UserID: u.ID(), ExpiresAt: exp}
	if err := r.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := r.SessionByHash(ctx, s.TokenHash)
	if err != nil || got.UserID != u.ID() || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("session = %+v, %v", got, err)
	}
	if err := r.DeleteSession(ctx, s.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SessionByHash(ctx, s.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("err = %v", err)
	}
}
