package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/insights/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) domain.Repository { return postgres.NewRepository(dbtest.New(t)) })
}

// The 0010 migration backfills the activity log from every event already in the outbox.
func TestBackfillFromOutbox(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO outbox (id, name, payload, created_at) VALUES
		(101, 'issue.created',        '{"id":"a0000000-0000-4000-8000-00000000000a","key":"KYB-12","type":"task","title":"t"}', '2026-05-04T09:00:00Z'),
		(102, 'issue.transitioned',   '{"id":"a0000000-0000-4000-8000-00000000000a","key":"KYB-12","from":"todo","to":"done"}', '2026-05-04T10:00:00Z'),
		(103, 'issue.sprint_changed', '{"id":"a0000000-0000-4000-8000-00000000000a","key":"KYB-12","from":"","to":"s-1"}', '2026-05-04T11:00:00Z'),
		(104, 'issue.estimated',      '{"id":"a0000000-0000-4000-8000-00000000000a","key":"KYB-12","from":null,"to":2.5}', '2026-05-04T12:00:00Z'),
		(105, 'comment.added',        '{"id":"c","issue_key":"KYB-12"}', '2026-05-04T13:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.MigrationSQL("0010_activity.sql")
	if _, err := pool.Exec(ctx, sql[indexOfBackfill(sql):]); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	got, _ := postgres.NewRepository(pool).ForProject(ctx, "KYB")
	if len(got) != 4 {
		t.Fatalf("backfilled = %+v", got)
	}
	if got[0].Kind != domain.KindCreated || !got[0].At.Equal(time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)) ||
		got[1].To != "done" || got[2].To != "s-1" || got[3].ToPoints == nil || *got[3].ToPoints != 25 || got[3].FromPoints != nil {
		t.Fatalf("backfill content = %+v", got)
	}
}

func indexOfBackfill(sql string) int {
	const marker = "-- backfill"
	for i := 0; i+len(marker) <= len(sql); i++ {
		if sql[i:i+len(marker)] == marker {
			return i
		}
	}
	return 0
}
