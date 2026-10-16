// Package memory stores automation rules in memory (tests, dev mode).
package memory

import (
	"context"
	"slices"
	"sort"
	"sync"

	"github.com/bakhod1r/kyber/internal/automation/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type stored struct {
	ws   string
	rule domain.Rule
}

type Repository struct {
	mu    sync.Mutex
	rules map[domain.RuleID]stored
	runs  []struct {
		ws  string
		run domain.Run
	}
}

func NewRepository() *Repository { return &Repository{rules: map[domain.RuleID]stored{}} }

func clone(r domain.Rule) *domain.Rule {
	r.Actions = slices.Clone(r.Actions)
	return &r
}

func (m *Repository) Save(ctx context.Context, r *domain.Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws := tenant.Workspace(ctx)
	cur, ok := m.rules[r.ID]
	if ok != (r.Version > 0) || (ok && (cur.ws != ws || cur.rule.Version != r.Version)) {
		return domain.ErrRuleConflict
	}
	r.Version++
	m.rules[r.ID] = stored{ws: ws, rule: *clone(*r)}
	return nil
}

func (m *Repository) ByID(ctx context.Context, id domain.RuleID) (*domain.Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.rules[id]
	if !ok || s.ws != tenant.Workspace(ctx) {
		return nil, domain.ErrRuleNotFound
	}
	return clone(s.rule), nil
}

func (m *Repository) ForProject(ctx context.Context, project string) ([]*domain.Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws := tenant.Workspace(ctx)
	var out []*domain.Rule
	for _, s := range m.rules {
		if s.ws == ws && s.rule.Project == project {
			out = append(out, clone(s.rule))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Repository) Delete(ctx context.Context, id domain.RuleID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.rules[id]; ok && s.ws == tenant.Workspace(ctx) {
		delete(m.rules, id)
	}
	return nil
}

func (m *Repository) Record(ctx context.Context, run domain.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs = append(m.runs, struct {
		ws  string
		run domain.Run
	}{tenant.Workspace(ctx), run})
	return nil
}

func (m *Repository) Runs(ctx context.Context, project string, limit int) ([]domain.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws := tenant.Workspace(ctx)
	var out []domain.Run
	for i := len(m.runs) - 1; i >= 0 && len(out) < limit; i-- {
		if r := m.runs[i]; r.ws == ws && r.run.Project == project {
			out = append(out, r.run)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
