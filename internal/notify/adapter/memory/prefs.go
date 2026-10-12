package memory

import (
	"context"
	"sync"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

type PrefsRepository struct {
	mu    sync.Mutex
	prefs map[domain.UserID]domain.Prefs
}

func NewPrefsRepository() *PrefsRepository {
	return &PrefsRepository{prefs: map[domain.UserID]domain.Prefs{}}
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
