package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

// Invitations is the PostgreSQL InvitationRepository (table project_invitations).
type Invitations struct{ pool *pgxpool.Pool }

func NewInvitations(pool *pgxpool.Pool) *Invitations { return &Invitations{pool: pool} }

const invitationCols = `id, workspace_id, project_key, email, role, invited_by, token_hash, created_at, expires_at,
	coalesce(accepted_by::text, ''), accepted_at`

func (r *Invitations) Save(ctx context.Context, i *domain.Invitation) error {
	s := i.Snapshot()
	var acceptedBy, acceptedAt any
	if s.AcceptedBy != "" {
		acceptedBy, acceptedAt = string(s.AcceptedBy), s.AcceptedAt
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if s.AcceptedBy == "" {
			if _, err := tx.Exec(ctx, `DELETE FROM project_invitations WHERE workspace_id = $1 AND project_key = $2 AND email = $3
				AND accepted_by IS NULL AND id <> $4`, string(s.Workspace), s.Project, s.Email, string(s.ID)); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO project_invitations (id, workspace_id, project_key, email, role, invited_by, token_hash,
			created_at, expires_at, accepted_by, accepted_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (id) DO UPDATE SET accepted_by = excluded.accepted_by, accepted_at = excluded.accepted_at`,
			string(s.ID), string(s.Workspace), s.Project, s.Email, string(s.Role), string(s.InvitedBy), s.TokenHash,
			s.CreatedAt, s.ExpiresAt, acceptedBy, acceptedAt)
		return err
	})
}

func (r *Invitations) ByTokenHash(ctx context.Context, hash string) (*domain.Invitation, error) {
	return one(r.pool.QueryRow(ctx, `SELECT `+invitationCols+` FROM project_invitations WHERE token_hash = $1`, hash))
}

func (r *Invitations) ByID(ctx context.Context, id domain.InvitationID) (*domain.Invitation, error) {
	return one(r.pool.QueryRow(ctx, `SELECT `+invitationCols+` FROM project_invitations WHERE id = $1 AND workspace_id = $2`,
		string(id), tenant.Workspace(ctx)))
}

func (r *Invitations) Pending(ctx context.Context, project string) ([]*domain.Invitation, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+invitationCols+` FROM project_invitations
		WHERE workspace_id = $1 AND project_key = $2 AND accepted_by IS NULL ORDER BY created_at DESC`, tenant.Workspace(ctx), project)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (*domain.Invitation, error) { return scanInvitation(row) })
}

func (r *Invitations) Delete(ctx context.Context, id domain.InvitationID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM project_invitations WHERE id = $1 AND workspace_id = $2`, string(id), tenant.Workspace(ctx))
	return err
}

func one(row pgx.Row) (*domain.Invitation, error) {
	i, err := scanInvitation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrInvitationNotFound
	}
	return i, err
}

func scanInvitation(row pgx.Row) (*domain.Invitation, error) {
	var s domain.InvitationSnapshot
	var acceptedAt *time.Time
	if err := row.Scan(&s.ID, &s.Workspace, &s.Project, &s.Email, &s.Role, &s.InvitedBy, &s.TokenHash, &s.CreatedAt, &s.ExpiresAt,
		&s.AcceptedBy, &acceptedAt); err != nil {
		return nil, err
	}
	s.CreatedAt, s.ExpiresAt = s.CreatedAt.UTC(), s.ExpiresAt.UTC()
	if acceptedAt != nil {
		s.AcceptedAt = acceptedAt.UTC()
	}
	return domain.RehydrateInvitation(s), nil
}
