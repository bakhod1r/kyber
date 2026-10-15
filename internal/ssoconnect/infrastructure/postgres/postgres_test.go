package postgres_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
	"github.com/bakhod1r/kyber/internal/platform/seal"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain/repotest"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/postgres"
)

func box(t *testing.T, b byte) *seal.Box {
	t.Helper()
	x, err := seal.New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestContract(t *testing.T) {
	repotest.Run(t, postgres.New(dbtest.New(t), box(t, 1)))
}

func TestSecretsAreSealed(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	s := postgres.New(pool, box(t, 1))
	c, _ := domain.NewConnection(repotest.C1, domain.Settings{Name: "Okta", Issuer: "https://acme.okta.com", ClientID: "a", ClientSecret: "top-secret"}, time.Now())
	if err := s.Save(ctx, c); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT client_secret FROM sso_connections`).Scan(&raw); err != nil || bytes.Contains(raw, []byte("top-secret")) {
		t.Fatalf("stored in clear: %q %v", raw, err)
	}
	if _, err := postgres.New(pool, box(t, 2)).ByID(ctx, repotest.C1); !errors.Is(err, seal.ErrOpen) {
		t.Fatalf("other key: %v", err)
	}
	noKey := postgres.New(pool, nil)
	if err := noKey.Save(ctx, c); !errors.Is(err, domain.ErrNoEncryptionKey) {
		t.Fatalf("save without key: %v", err)
	}
	if _, err := noKey.All(ctx); !errors.Is(err, domain.ErrNoEncryptionKey) {
		t.Fatalf("read without key: %v", err)
	}
}
