package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type PrefsRepository struct {
	mu     sync.Mutex
	prefs  map[domain.UserID]domain.Prefs
	digest map[domain.UserID]time.Time // last digest sent
}

func NewPrefsRepository() *PrefsRepository {
	return &PrefsRepository{prefs: map[domain.UserID]domain.Prefs{}, digest: map[domain.UserID]time.Time{}}
}

func (r *PrefsRepository) DigestSubscribers(context.Context) ([]domain.DigestSubscriber, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.DigestSubscriber
	for u, p := range r.prefs {
		if p.EmailDigest {
			out = append(out, domain.DigestSubscriber{User: u, Prefs: p, LastSent: r.digest[u]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].User < out[j].User })
	return out, nil
}

func (r *PrefsRepository) MarkDigestSent(_ context.Context, user domain.UserID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.digest[user] = at.UTC()
	return nil
}

func (r *PrefsRepository) Prefs(_ context.Context, user domain.UserID) (domain.Prefs, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.prefs[user]; ok {
		return p, nil
	}
	return domain.DefaultPrefs(), nil
}

func (r *PrefsRepository) SetPrefs(_ context.Context, user domain.UserID, p domain.Prefs) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefs[user] = p
	return nil
}
