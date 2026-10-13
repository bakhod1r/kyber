package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type historyKey struct {
	ws    string
	event int64
	seq   int
}

// HistoryRepository keeps issue history in memory.
type HistoryRepository struct {
	mu      sync.Mutex
	entries map[historyKey]domain.HistoryEntry
}

func NewHistoryRepository() *HistoryRepository {
	return &HistoryRepository{entries: map[historyKey]domain.HistoryEntry{}}
}

func (r *HistoryRepository) Record(ctx context.Context, entries ...domain.HistoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range entries {
		k := historyKey{tenant.Workspace(ctx), e.Event, e.Seq}
		if _, ok := r.entries[k]; !ok {
			r.entries[k] = e
		}
	}
	return nil
}

func (r *HistoryRepository) ForIssue(ctx context.Context, issue domain.IssueID) ([]domain.HistoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.HistoryEntry{}
	for k, e := range r.entries {
		if k.ws == tenant.Workspace(ctx) && e.Issue == issue {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Event != b.Event {
			return a.Event < b.Event
		}
		return a.Seq < b.Seq
	})
	return out, nil
}
