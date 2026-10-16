package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/project/domain"
)

// Invitations is the in-memory InvitationRepository.
type Invitations struct {
	mu   sync.Mutex
	byID map[domain.InvitationID]domain.InvitationSnapshot
}

func NewInvitations() *Invitations {
	return &Invitations{byID: map[domain.InvitationID]domain.InvitationSnapshot{}}
}

func (r *Invitations) Save(_ context.Context, i *domain.Invitation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := i.Snapshot()
	if s.AcceptedBy == "" {
		for id, o := range r.byID {
			if id != s.ID && o.AcceptedBy == "" && o.Workspace == s.Workspace && o.Project == s.Project && o.Email == s.Email {
				delete(r.byID, id)
			}
		}
	}
	r.byID[s.ID] = s
	return nil
}

func (r *Invitations) ByTokenHash(_ context.Context, hash string) (*domain.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.byID {
		if s.TokenHash == hash {
			return domain.RehydrateInvitation(s), nil
		}
	}
	return nil, domain.ErrInvitationNotFound
}

func (r *Invitations) ByID(ctx context.Context, id domain.InvitationID) (*domain.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[id]
	if !ok || string(s.Workspace) != tenant.Workspace(ctx) {
		return nil, domain.ErrInvitationNotFound
	}
	return domain.RehydrateInvitation(s), nil
}

func (r *Invitations) Pending(ctx context.Context, project string) ([]*domain.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Invitation
	for _, s := range r.byID {
		if s.AcceptedBy == "" && s.Project == project && string(s.Workspace) == tenant.Workspace(ctx) {
			out = append(out, domain.RehydrateInvitation(s))
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt().After(out[b].CreatedAt()) })
	return out, nil
}

func (r *Invitations) Delete(ctx context.Context, id domain.InvitationID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.byID[id]; ok && string(s.Workspace) == tenant.Workspace(ctx) {
		delete(r.byID, id)
	}
	return nil
}
