package postgres

import (
	"context"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

func (r *Repository) SaveUser(ctx context.Context, u *domain.User) error {
	tag, err := r.pool.Exec(ctx, `UPDATE users SET name = $2, active = $3 WHERE id = $1`, string(u.ID()), u.Name(), u.Active())
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return err
}

func (r *Repository) ListUsers(ctx context.Context, email domain.Email, offset, limit int) ([]*domain.User, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE $1 = '' OR email = $1`, email.String()).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, userCols+` WHERE $1 = '' OR email = $1 ORDER BY email OFFSET $2 LIMIT $3`, email.String(), offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (r *Repository) DeleteSessionsOf(ctx context.Context, id domain.UserID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, string(id))
	return err
}

var _ domain.Directory = (*Repository)(nil)
