package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type linkRow struct {
	ws string
	l  domain.Link
}

// LinkRepository keeps issue links in memory and writes events to the issue repository's outbox.
type LinkRepository struct {
	mu     sync.Mutex
	issues *Repository
	rows   []linkRow
}

func NewLinkRepository(issues *Repository) *LinkRepository { return &LinkRepository{issues: issues} }

func (r *LinkRepository) Add(ctx context.Context, l domain.Link, events []domain.Event) error {
	r.mu.Lock()
	for _, x := range r.rows {
		if x.ws == tenant.Workspace(ctx) && x.l.SameAs(l) {
			r.mu.Unlock()
			return domain.ErrLinkExists
		}
	}
	r.rows = append(r.rows, linkRow{ws: tenant.Workspace(ctx), l: l})
	r.mu.Unlock()
	r.issues.appendOutbox(ctx, events)
	return nil
}

func (r *LinkRepository) Remove(ctx context.Context, id domain.LinkID, events []domain.Event) error {
	r.mu.Lock()
	for i, x := range r.rows {
		if x.ws == tenant.Workspace(ctx) && x.l.ID == id {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			r.mu.Unlock()
			r.issues.appendOutbox(ctx, events)
			return nil
		}
	}
	r.mu.Unlock()
	return domain.ErrLinkNotFound
}

func (r *LinkRepository) ByID(ctx context.Context, id domain.LinkID) (domain.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.rows {
		if x.ws == tenant.Workspace(ctx) && x.l.ID == id {
			return x.l, nil
		}
	}
	return domain.Link{}, domain.ErrLinkNotFound
}

func (r *LinkRepository) ForIssue(ctx context.Context, issue domain.IssueID) ([]domain.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Link{}
	for _, x := range r.rows {
		if x.ws == tenant.Workspace(ctx) && (x.l.From == issue || x.l.To == issue) {
			out = append(out, x.l)
		}
	}
	return out, nil
}
