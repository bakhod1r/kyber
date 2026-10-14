package memory

import (
	"context"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

type visit struct {
	user  domain.UserID
	until time.Time
}

// PresenceRegistry keeps who views which issue in this process only (issue ids are globally unique).
// It is the production adapter too: presence is ephemeral and per replica.
type PresenceRegistry struct {
	mu   sync.Mutex
	byIs map[domain.IssueID][]visit // in order of arrival
}

func NewPresenceRegistry() *PresenceRegistry {
	return &PresenceRegistry{byIs: map[domain.IssueID][]visit{}}
}

func (r *PresenceRegistry) Seen(_ context.Context, issue domain.IssueID, user domain.UserID, now, until time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	vs := r.prune(issue, now)
	for i := range vs {
		if vs[i].user == user {
			vs[i].until = until
			return false, nil
		}
	}
	r.byIs[issue] = append(vs, visit{user: user, until: until})
	return true, nil
}

func (r *PresenceRegistry) Present(_ context.Context, issue domain.IssueID, now time.Time) ([]domain.UserID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	vs := r.prune(issue, now)
	out := make([]domain.UserID, len(vs))
	for i, v := range vs {
		out[i] = v.user
	}
	return out, nil
}

// prune drops the issue's expired visits (callers hold mu).
func (r *PresenceRegistry) prune(issue domain.IssueID, now time.Time) []visit {
	live := []visit{}
	for _, v := range r.byIs[issue] {
		if v.until.After(now) {
			live = append(live, v)
		}
	}
	if len(live) == 0 {
		delete(r.byIs, issue)
	} else {
		r.byIs[issue] = live
	}
	return live
}
