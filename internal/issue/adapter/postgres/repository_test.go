package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/adapter/postgres"
	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber'),
			('10000000-0000-4000-8000-000000000002','OPS','Ops')`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO sprints (id, project_key, name, state) VALUES ($1, 'KYB', 'Sprint A', 'planned')`, repotest.SprintA); err != nil {
			t.Fatal(err)
		}
		outbox := outboxReader(t, pool)
		return postgres.NewRepository(pool), outbox
	})
}

func TestCommentContract(t *testing.T) {
	repotest.RunComments(t, func(t *testing.T) (domain.CommentRepository, *domain.Issue, func() []string) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber')`); err != nil {
			t.Fatal(err)
		}
		issues := postgres.NewRepository(pool)
		key, _ := domain.NewIssueKey("KYB", 1)
		is, _ := domain.NewIssue("00000000-0000-4000-8000-000000000001", key, "t", domain.TypeTask, domain.DefaultWorkflow())
		if err := issues.Save(ctx, is, nil); err != nil {
			t.Fatal(err)
		}
		return postgres.NewCommentRepository(pool), is, outboxReader(t, pool)
	})
}

func outboxReader(t *testing.T, pool *pgxpool.Pool) func() []string {
	ctx := context.Background()
	return func() []string {
		rows, err := pool.Query(ctx, `SELECT name FROM outbox ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var names []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			names = append(names, n)
		}
		return names
	}
}
