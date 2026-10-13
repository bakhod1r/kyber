package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/announcement/domain"
	"github.com/bakhod1r/kyber/internal/announcement/domain/repotest"
	"github.com/bakhod1r/kyber/internal/announcement/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/auth"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

func TestContract(t *testing.T) { repotest.Run(t, memory.NewRepository()) }

func TestOutboxRecordsEventsWithTheirEnvelope(t *testing.T) {
	repo := memory.NewRepository()
	ctx := auth.WithActor(tenant.With(context.Background(), "w-1"), "u-1")
	a, _ := domain.Publish("a-1", "w-1", "", "Hi", "", "u-1", time.Now())
	_ = repo.Save(ctx, a)
	if ev, meta := repo.Outbox(), repo.OutboxMeta(); len(ev) != 1 || len(meta) != 1 || meta[0].Actor != "u-1" || meta[0].Workspace != "w-1" {
		t.Fatalf("outbox = %v %v", ev, meta)
	}
}
