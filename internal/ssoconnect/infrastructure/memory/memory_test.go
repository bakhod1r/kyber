package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"

	"github.com/bakhod1r/kyber/internal/ssoconnect/domain/repotest"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/memory"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.New()) }

func TestSameTimeOrdersByID(t *testing.T) {
	s := memory.New()
	at := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	for _, id := range []domain.ConnectionID{"b", "a"} {
		c, _ := domain.NewConnection(id, domain.Settings{Name: "X", Issuer: "https://x.example", ClientID: "c", ClientSecret: "s"}, at)
		_ = s.Save(context.Background(), c)
	}
	if all, _ := s.All(context.Background()); all[0].ID != "a" {
		t.Fatalf("order = %v", all)
	}
}

func TestRequiredByOrdersByID(t *testing.T) {
	s := memory.New()
	ctx := context.Background()
	for _, id := range []domain.ConnectionID{"b", "a"} {
		c, _ := domain.NewConnection(id, domain.Settings{Name: "X", Issuer: "https://x.example", ClientID: "c", ClientSecret: "s", Workspace: "w-1"}, time.Now())
		_ = c.Enforce(true)
		_ = s.Save(ctx, c)
	}
	if req, _ := s.RequiredBy(ctx, "w-1"); len(req) != 2 || req[0].ID != "a" {
		t.Fatalf("required = %v", req)
	}
}
