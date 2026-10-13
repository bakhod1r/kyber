package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/calendar/domain/repotest"
	"github.com/bakhod1r/kyber/internal/calendar/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	pool := dbtest.New(t)
	for _, u := range []string{repotest.Lead, repotest.Dev, repotest.Other} {
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'U', '')`, u, u+"@x.uz"); err != nil {
			t.Fatal(err)
		}
	}
	repotest.Run(t, postgres.NewRepository(pool))
	var names []string
	rows, _ := pool.Query(context.Background(), `SELECT name FROM outbox ORDER BY id`)
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	if strings.Join(names, ",") != "meeting.scheduled,meeting.scheduled,meeting.reminder,meeting.cancelled" {
		t.Fatalf("outbox = %v", names)
	}
}
