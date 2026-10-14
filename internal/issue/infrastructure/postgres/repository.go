// Package postgres is the PostgreSQL Issue repository with a transactional outbox.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/bakhod1r/kyber/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/db"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Save(ctx context.Context, is *domain.Issue, events []domain.Event) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var tag interface{ RowsAffected() int64 }
		tt := is.TimeTracking()
		start, due := is.Schedule()
		fields := is.Fields()
		if fields == nil {
			fields = map[domain.FieldID]string{}
		}
		var err error
		if is.Version() == 0 {
			tag, err = tx.Exec(ctx, `INSERT INTO issues (id, project_key, number, title, type, status,
				description, priority, assignee_id, rank, sprint_id, reporter_id, estimate_tenths, version, workspace_id, parent_id, security_level, resolution, original_estimate_s, remaining_estimate_s, time_spent_s, custom_fields, start_date, due_date, labels)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, $14, $15, $16, $17, $18, $19, $20, $21, NULLIF($22, '')::date, NULLIF($23, '')::date, $24) ON CONFLICT DO NOTHING`,
				string(is.ID()), is.Key().Project(), is.Key().Number(), is.Title(), string(is.Type()), string(is.Status()),
				is.Description(), string(is.Priority()), nullable(string(is.Assignee())), string(is.Rank()),
				nullable(string(is.Sprint())), nullable(string(is.Reporter())), estimateCol(is), tenant.Workspace(ctx), nullable(string(is.Parent())), is.SecurityLevel(), string(is.Resolution()), tt.Original, tt.Remaining, tt.Spent, fields, string(start), string(due), labelsCol(is))
		} else {
			tag, err = tx.Exec(ctx, `UPDATE issues SET title = $2, type = $3, status = $4, description = $5,
				priority = $6, assignee_id = $7, rank = $8, sprint_id = $9, estimate_tenths = $11,
				parent_id = $13, security_level = $14, resolution = $15, original_estimate_s = $16, remaining_estimate_s = $17, time_spent_s = $18, custom_fields = $19, start_date = NULLIF($20, '')::date, due_date = NULLIF($21, '')::date, labels = $22, version = version + 1, updated_at = now()
				WHERE id = $1 AND version = $10 AND workspace_id = $12`,
				string(is.ID()), is.Title(), string(is.Type()), string(is.Status()), is.Description(),
				string(is.Priority()), nullable(string(is.Assignee())), string(is.Rank()), nullable(string(is.Sprint())),
				is.Version(), estimateCol(is), tenant.Workspace(ctx), nullable(string(is.Parent())), is.SecurityLevel(), string(is.Resolution()), tt.Original, tt.Remaining, tt.Spent, fields, string(start), string(due), labelsCol(is))
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return domain.ErrConcurrentModification
		}
		if err := saveFixVersions(ctx, tx, is); err != nil {
			return err
		}
		return db.WriteOutbox(ctx, tx, events)
	})
	if err != nil {
		return err
	}
	is.MarkPersisted()
	return nil
}

const selectCols = `SELECT id::text, project_key, number, title, type, status, description, priority,
	COALESCE(assignee_id::text, ''), rank, COALESCE(sprint_id::text, ''), COALESCE(reporter_id::text, ''), estimate_tenths, version,
	COALESCE(parent_id::text, ''), COALESCE((SELECT p.number FROM issues p WHERE p.id = issues.parent_id), 0), security_level, resolution, original_estimate_s, remaining_estimate_s, time_spent_s, custom_fields,
	COALESCE(to_char(start_date, 'YYYY-MM-DD'), ''), COALESCE(to_char(due_date, 'YYYY-MM-DD'), ''),
	ARRAY(SELECT f.version_id::text FROM issue_fix_versions f WHERE f.issue_id = issues.id ORDER BY 1), labels FROM issues`

func (r *Repository) ByKey(ctx context.Context, key domain.IssueKey) (*domain.Issue, error) {
	is, err := scan(r.pool.QueryRow(ctx, selectCols+` WHERE workspace_id = $3 AND project_key = $1 AND number = $2`, key.Project(), key.Number(), tenant.Workspace(ctx)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrIssueNotFound
	}
	return is, err
}

func (r *Repository) ListByProject(ctx context.Context, project string, f domain.ListFilter) ([]*domain.Issue, error) {
	where, args := `WHERE workspace_id = $2 AND project_key = $1`, []any{project, tenant.Workspace(ctx)}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		where += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	if f.Sprint != nil {
		if *f.Sprint == "" {
			where += ` AND sprint_id IS NULL`
		} else {
			args = append(args, string(*f.Sprint))
			where += fmt.Sprintf(` AND sprint_id = $%d`, len(args))
		}
	}
	if f.Parent != nil {
		args = append(args, string(*f.Parent))
		where += fmt.Sprintf(` AND parent_id = $%d`, len(args))
	}
	if f.FixVersion != nil {
		if !kid.Valid(string(*f.FixVersion)) {
			return nil, nil
		}
		args = append(args, string(*f.FixVersion))
		where += fmt.Sprintf(` AND id IN (SELECT issue_id FROM issue_fix_versions WHERE version_id = $%d)`, len(args))
	}
	if f.Label != nil {
		args = append(args, string(*f.Label))
		where += fmt.Sprintf(` AND $%d = ANY(labels)`, len(args))
	}
	rows, err := r.pool.Query(ctx, selectCols+" "+where+` ORDER BY rank, number`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Issue
	for rows.Next() {
		is, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, rows.Err()
}

func (r *Repository) LastRank(ctx context.Context, project string) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE workspace_id = $2 AND project_key = $1 ORDER BY rank DESC LIMIT 1`, project, tenant.Workspace(ctx))
}

func (r *Repository) NextRank(ctx context.Context, project string, after domain.Rank) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE workspace_id = $3 AND project_key = $1 AND rank > $2 ORDER BY rank LIMIT 1`, project, string(after), tenant.Workspace(ctx))
}

func (r *Repository) PrevRank(ctx context.Context, project string, before domain.Rank) (domain.Rank, error) {
	return r.rankQuery(ctx, `SELECT rank FROM issues WHERE workspace_id = $3 AND project_key = $1 AND rank < $2 ORDER BY rank DESC LIMIT 1`, project, string(before), tenant.Workspace(ctx))
}

func (r *Repository) rankQuery(ctx context.Context, sql string, args ...any) (domain.Rank, error) {
	var rank string
	err := r.pool.QueryRow(ctx, sql, args...).Scan(&rank)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return domain.Rank(rank), err
}

func scan(row pgx.Row) (*domain.Issue, error) {
	var s domain.Snapshot
	var project string
	var number, parentNumber int
	var fixVersions, labels []string
	if err := row.Scan(&s.ID, &project, &number, &s.Title, &s.Type, &s.Status, &s.Description,
		&s.Priority, &s.Assignee, &s.Rank, &s.Sprint, &s.Reporter, &s.Estimate, &s.Version, &s.Parent, &parentNumber, &s.Security, &s.Resolution, &s.Time.Original, &s.Time.Remaining, &s.Time.Spent, &s.Fields, &s.StartDate, &s.DueDate, &fixVersions, &labels); err != nil {
		return nil, err
	}
	for _, v := range fixVersions {
		s.FixVersions = append(s.FixVersions, domain.VersionID(v))
	}
	for _, l := range labels {
		s.Labels = append(s.Labels, domain.Label(l))
	}
	if s.Parent != "" {
		s.ParentKey, _ = domain.NewIssueKey(project, parentNumber) // parents are in the same project
	}
	key, err := domain.NewIssueKey(project, number)
	if err != nil {
		return nil, err
	}
	s.Key = key
	return domain.Rehydrate(s), nil
}

func estimateCol(is *domain.Issue) *int {
	p, ok := is.Estimate()
	if !ok {
		return nil
	}
	v := int(p)
	return &v
}

// nullable maps the domain's "" (unassigned / backlog) to SQL NULL.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// saveFixVersions replaces the issue's fix versions (issue_fix_versions rows) in the transaction.
func saveFixVersions(ctx context.Context, tx pgx.Tx, is *domain.Issue) error {
	ids := make([]string, 0, len(is.FixVersions()))
	for _, v := range is.FixVersions() {
		ids = append(ids, string(v))
	}
	if _, err := tx.Exec(ctx, `DELETE FROM issue_fix_versions WHERE issue_id = $1 AND NOT (version_id::text = ANY($2))`,
		string(is.ID()), ids); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO issue_fix_versions (issue_id, version_id)
		SELECT $1, v::uuid FROM unnest($2::text[]) v ON CONFLICT DO NOTHING`, string(is.ID()), ids)
	return err
}

// labelsCol is the issue's labels as a text[] (never NULL).
func labelsCol(is *domain.Issue) []string {
	out := make([]string, 0, len(is.Labels()))
	for _, l := range is.Labels() {
		out = append(out, string(l))
	}
	return out
}
