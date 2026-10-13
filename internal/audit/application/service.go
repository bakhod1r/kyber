// Package app holds the Audit context use cases.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/audit/domain"
)

var (
	ErrNotFound  = errors.New("project not found")
	ErrForbidden = errors.New("only project admins can read the audit log")
)

// Admins checks project administration (ACL to the Project context): ErrNotFound for
// non-members, ErrForbidden for members who are not admins.
type Admins interface {
	Administer(ctx context.Context, actor, project string) error
}

// Names resolves display names (ACL to Identity).
type Names interface {
	DisplayName(ctx context.Context, id string) (string, error)
}

// Line is an entry with the actor's name.
type Line struct {
	domain.Entry
	ActorName string
}

type Service struct {
	repo   domain.Repository
	admins Admins
	names  Names
	now    func() time.Time
}

func NewService(repo domain.Repository, admins Admins, names Names, now func() time.Time) *Service {
	return &Service{repo: repo, admins: admins, names: names, now: now}
}

// Record appends an entry for a change that has just succeeded.
func (s *Service) Record(ctx context.Context, actor, action, target, project string, status int) error {
	e, err := domain.NewEntry(actor, action, target, project, status, s.now())
	if err != nil {
		return err
	}
	return s.repo.Append(ctx, e)
}

// DefaultLimit is how many entries Log returns when no limit is given.
const DefaultLimit = 200

// Log returns the project's newest audit entries to project admins.
func (s *Service) Log(ctx context.Context, actor, project string, limit int) ([]Line, error) {
	if err := s.admins.Administer(ctx, actor, project); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > DefaultLimit {
		limit = DefaultLimit
	}
	entries, err := s.repo.ForProject(ctx, project, limit)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	out := make([]Line, 0, len(entries))
	for _, e := range entries {
		n, ok := names[e.Actor]
		if !ok {
			if n, err = s.names.DisplayName(ctx, e.Actor); err != nil {
				return nil, err
			}
			names[e.Actor] = n
		}
		out = append(out, Line{Entry: e, ActorName: n})
	}
	return out, nil
}
