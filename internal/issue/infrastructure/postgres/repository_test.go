package postgres_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/issue/domain/repotest"
	"github.com/bakhod1r/kyber/internal/issue/infrastructure/postgres"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/db/dbtest"
)

func TestContract(t *testing.T) {
	repotest.Run(t, func(t *testing.T) (domain.Repository, func() []string) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001'),
			('10000000-0000-4000-8000-000000000002','OPS','Ops','00000000-0000-4000-8000-000000000001')`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO sprints (id, project_key, name, state, workspace_id) VALUES ($1, 'KYB', 'Sprint A', 'planned', '00000000-0000-4000-8000-000000000001')`, repotest.SprintA); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO versions (id, workspace_id, project_key, name, state, version) VALUES
			($1, '00000000-0000-4000-8000-000000000001', 'KYB', '1.0', 'unreleased', 1),
			($2, '00000000-0000-4000-8000-000000000001', 'KYB', '2.0', 'unreleased', 1)`, repotest.VersionA, repotest.VersionB); err != nil {
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
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		issues := postgres.NewRepository(pool)
		key, _ := domain.NewIssueKey("KYB", 1)
		is, _ := domain.NewIssue("00000000-0000-4000-8000-000000000001", key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
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

func TestWorkflowContract(t *testing.T) {
	repotest.RunWorkflows(t, func(t *testing.T) domain.WorkflowRepository {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001'),
			('10000000-0000-4000-8000-000000000002','KYB','Kyber too',$1)`, repotest.OtherWorkspace); err != nil {
			t.Fatal(err)
		}
		return postgres.NewWorkflowRepository(pool)
	})
}

func TestHistoryContract(t *testing.T) {
	repotest.RunHistory(t, func(t *testing.T) domain.HistoryRepository {
		pool := dbtest.New(t)
		if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		return postgres.NewHistoryRepository(pool)
	})
}

// Migration 0021 rebuilds every past action from the outbox (events are never deleted).
func TestHistoryBackfillFromOutbox(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	const issue = `"id":"a0000000-0000-4000-8000-00000000000a","key":"KYB-12"`
	_, err := pool.Exec(ctx, `INSERT INTO outbox (id, name, payload, created_at) VALUES
		(101, 'issue.created',        '{`+issue+`,"type":"task","title":"Login","reporter":"u-r"}', '2026-05-04T09:00:00Z'),
		(102, 'issue.transitioned',   '{`+issue+`,"from":"todo","to":"done"}', '2026-05-04T10:00:00Z'),
		(103, 'issue.assigned',       '{`+issue+`,"from":"","to":"u-a","by":"u-r"}', '2026-05-04T11:00:00Z'),
		(104, 'issue.edited',         '{`+issue+`,"fields":["title","priority"]}', '2026-05-04T12:00:00Z'),
		(105, 'issue.estimated',      '{`+issue+`,"from":null,"to":2.5}', '2026-05-04T13:00:00Z'),
		(106, 'comment.added',        '{"id":"c","issue_id":"a0000000-0000-4000-8000-00000000000a","issue_key":"KYB-12","author":"u-a","body":"hi"}', '2026-05-04T14:00:00Z'),
		(107, 'sprint.started',       '{"id":"s"}', '2026-05-04T15:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.MigrationSQL("0021_issue_history.sql")
	if _, err := pool.Exec(ctx, sql[strings.Index(sql, "-- Backfill"):]); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	got, _ := postgres.NewHistoryRepository(pool).ForIssue(ctx, "a0000000-0000-4000-8000-00000000000a")
	var lines []string
	for _, e := range got {
		lines = append(lines, string(e.By)+"|"+e.Field+"|"+e.From+"|"+e.To)
	}
	want := "u-r|created||Login,|status|todo|done,u-r|assignee||u-a,|title||,|priority||,|estimate||2.5,u-a|comment||hi"
	if strings.Join(lines, ",") != want {
		t.Fatalf("backfilled:\n%s\nwant:\n%s", strings.Join(lines, ","), want)
	}
}

func TestLinkContract(t *testing.T) {
	repotest.RunLinks(t, func(t *testing.T) (domain.LinkRepository, [3]*domain.Issue, func() []string) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		issues := postgres.NewRepository(pool)
		var is [3]*domain.Issue
		for n := range 3 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
			if err := issues.Save(ctx, is[n], nil); err != nil {
				t.Fatal(err)
			}
		}
		return postgres.NewLinkRepository(pool), is, outboxReader(t, pool)
	})
}

func TestWatcherContract(t *testing.T) {
	repotest.RunWatchers(t, func(t *testing.T) (domain.WatcherRepository, [2]*domain.Issue) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h'), ($2, 'w@x.uz', 'W', 'h')`,
			repotest.Assignee, repotest.Watcher); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		issues := postgres.NewRepository(pool)
		var is [2]*domain.Issue
		for n := range 2 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
			if err := issues.Save(ctx, is[n], nil); err != nil {
				t.Fatal(err)
			}
		}
		return postgres.NewWatcherRepository(pool), is
	})
}

func TestWorklogContract(t *testing.T) {
	repotest.RunWorklogs(t, func(t *testing.T) (domain.WorklogRepository, [2]*domain.Issue) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		issues := postgres.NewRepository(pool)
		var is [2]*domain.Issue
		for n := range 2 {
			key, _ := domain.NewIssueKey("KYB", n+1)
			is[n], _ = domain.NewIssue(domain.IssueID(fmt.Sprintf("00000000-0000-4000-8000-00000000000%d", n+1)), key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
			if err := issues.Save(ctx, is[n], nil); err != nil {
				t.Fatal(err)
			}
		}
		return postgres.NewWorklogRepository(pool), is
	})
}

func TestReactionContract(t *testing.T) {
	repotest.RunReactions(t, func(t *testing.T) (domain.ReactionRepository, [2]domain.CommentID) {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, name, password_hash) VALUES ($1, 'a@x.uz', 'A', 'h')`, repotest.Assignee); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES ('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001')`); err != nil {
			t.Fatal(err)
		}
		key, _ := domain.NewIssueKey("KYB", 1)
		is, _ := domain.NewIssue("00000000-0000-4000-8000-000000000001", key, "t", domain.TypeTask, repotest.Assignee, domain.DefaultWorkflow())
		if err := postgres.NewRepository(pool).Save(ctx, is, nil); err != nil {
			t.Fatal(err)
		}
		comments := postgres.NewCommentRepository(pool)
		var ids [2]domain.CommentID
		for n := range 2 {
			c, _ := domain.NewComment(domain.CommentID(fmt.Sprintf("00000000-0000-4000-8000-00000000001%d", n+1)), is.ID(), is.Key(), repotest.Assignee, "c", time.Now())
			if err := comments.Add(ctx, c, nil); err != nil {
				t.Fatal(err)
			}
			ids[n] = c.ID()
		}
		return postgres.NewReactionRepository(pool), ids
	})
}

func TestCustomFieldContract(t *testing.T) {
	repotest.RunCustomFields(t, func(t *testing.T) domain.CustomFieldRepository {
		pool := dbtest.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'other', 'Other')`, repotest.OtherWorkspace); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO projects (id, key, name, workspace_id) VALUES
			('10000000-0000-4000-8000-000000000001','KYB','Kyber','00000000-0000-4000-8000-000000000001'),
			('10000000-0000-4000-8000-000000000002','OPS','Ops','00000000-0000-4000-8000-000000000001'),
			('10000000-0000-4000-8000-000000000003','KYB','Kyber',$1)`, repotest.OtherWorkspace); err != nil {
			t.Fatal(err)
		}
		return postgres.NewCustomFieldRepository(pool)
	})
}
