// Package application holds the Search context's use cases: running JQL over the issues the actor
// may see, and saved filters.
package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/bakhod1r/kyber/internal/search/domain"
)

var (
	// ErrForbidden: only a filter's owner may change or delete it.
	ErrForbidden = errors.New("only the filter's owner may change it")
)

// Ports to the other contexts (anti-corruption layers live in infrastructure).
type (
	// Projects lists the projects the actor may browse (ACL to Project/Access).
	Projects interface {
		Browsable(ctx context.Context, actor string) ([]string, error)
	}
	// Issues returns what a query sees of the project's issues the actor may see (ACL to Issue Tracking).
	Issues interface {
		// Facts lists a project's visible issues; the store may drop those outside the narrow.
		Facts(ctx context.Context, actor, project string, n domain.Narrow) ([]domain.Issue, error)
		// CustomFields names the project's custom fields (lower-case name → type).
		CustomFields(ctx context.Context, project string) (map[string]domain.CustomType, error)
	}
	// Users finds users by email (ACL to Identity).
	Users interface {
		UserIDByEmail(ctx context.Context, email string) (string, bool, error)
	}
	// Planning names sprints and versions (ACL to Agile).
	Planning interface {
		Sprints(ctx context.Context, project string) ([]Sprint, error)
		Versions(ctx context.Context, project string) ([]NamedID, error)
	}
)

// NamedID is a sprint or version as queries see it.
type NamedID struct{ ID, Name string }

type Sprint struct {
	NamedID
	Open bool // planned or active
}

type Deps struct {
	Projects Projects
	Issues   Issues
	Users    Users
	Planning Planning
	Filters  domain.FilterRepository
	NewID    func() string
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// MaxResults bounds one page of results.
const MaxResults = 200

// Result is a page of matching issues and how many matched in total.
type Result struct {
	Issues []domain.Issue
	Total  int
}

// Search runs JQL over every project the actor may browse; hidden issues never match.
func (s *Service) Search(ctx context.Context, actor, jql string, limit int) (Result, error) {
	projects, err := s.d.Projects.Browsable(ctx, actor)
	if err != nil {
		return Result{}, err
	}
	custom := map[string]domain.CustomType{}
	for _, p := range projects {
		fields, err := s.d.Issues.CustomFields(ctx, p)
		if err != nil {
			return Result{}, err
		}
		for name, t := range fields {
			if _, seen := custom[name]; !seen {
				custom[name] = t
			}
		}
	}
	q, err := domain.ParseWith(jql, custom)
	if err != nil {
		return Result{}, err
	}
	narrow := q.Narrow()
	if narrow.Projects != nil { // only the projects the query names are read at all
		kept := projects[:0:0]
		for _, p := range projects {
			if slices.Contains(narrow.Projects, strings.ToLower(p)) {
				kept = append(kept, p)
			}
		}
		projects = kept
	}
	var facts []domain.Issue
	r := &resolver{ctx: ctx, s: s, actor: actor, users: map[string]userHit{}}
	for _, p := range projects {
		list, err := s.d.Issues.Facts(ctx, actor, p, narrow)
		if err != nil {
			return Result{}, err
		}
		facts = append(facts, list...)
		sprints, err := s.d.Planning.Sprints(ctx, p)
		if err != nil {
			return Result{}, err
		}
		r.sprints = append(r.sprints, sprints...)
		versions, err := s.d.Planning.Versions(ctx, p)
		if err != nil {
			return Result{}, err
		}
		r.versions = append(r.versions, versions...)
	}
	out, _ := q.Run(facts, r)
	if r.err != nil {
		return Result{}, r.err
	}
	if limit <= 0 || limit > MaxResults {
		limit = MaxResults
	}
	res := Result{Issues: out, Total: len(out)}
	if len(out) > limit {
		res.Issues = out[:limit]
	}
	return res, nil
}

type userHit struct {
	id string
	ok bool
}

// resolver answers the query's name lookups; the first port error is kept and reported.
type resolver struct {
	ctx      context.Context
	s        *Service
	actor    string
	sprints  []Sprint
	versions []NamedID
	users    map[string]userHit
	err      error
}

func (r *resolver) User(name string) (string, bool) {
	key := strings.ToLower(name)
	if h, ok := r.users[key]; ok {
		return h.id, h.ok
	}
	id, ok, err := r.s.d.Users.UserIDByEmail(r.ctx, name)
	if err != nil && r.err == nil {
		r.err = err
	}
	r.users[key] = userHit{id, ok}
	return id, ok
}

func (r *resolver) CurrentUser() string { return r.actor }

func (r *resolver) Sprints(name string) []string {
	var out []string
	for _, sp := range r.sprints {
		if strings.EqualFold(sp.Name, name) {
			out = append(out, sp.ID)
		}
	}
	return out
}

func (r *resolver) OpenSprints() []string {
	var out []string
	for _, sp := range r.sprints {
		if sp.Open {
			out = append(out, sp.ID)
		}
	}
	return out
}

func (r *resolver) Versions(name string) []string {
	var out []string
	for _, v := range r.versions {
		if strings.EqualFold(v.Name, name) {
			out = append(out, v.ID)
		}
	}
	return out
}

// ---- saved filters

// Filters lists the actor's filters and the workspace's shared ones.
func (s *Service) Filters(ctx context.Context, actor string) ([]*domain.Filter, error) {
	return s.d.Filters.FiltersFor(ctx, actor)
}

// SaveFilter creates a filter owned by the actor.
func (s *Service) SaveFilter(ctx context.Context, actor, name, jql string, shared bool) (*domain.Filter, error) {
	f, err := domain.NewFilter(domain.FilterID(s.d.NewID()), actor, name, jql, shared)
	if err != nil {
		return nil, err
	}
	return f, s.d.Filters.SaveFilter(ctx, f)
}

// owned loads a filter the actor may see; only its owner may change it.
func (s *Service) owned(ctx context.Context, actor, id string) (*domain.Filter, error) {
	f, err := s.d.Filters.FilterByID(ctx, domain.FilterID(id))
	if err != nil {
		return nil, err
	}
	if f.Owner != actor {
		if !f.Shared {
			return nil, domain.ErrFilterNotFound // private filters stay private
		}
		return nil, ErrForbidden
	}
	return f, nil
}

// UpdateFilter changes a filter loaded at version (owner only).
func (s *Service) UpdateFilter(ctx context.Context, actor, id string, version int, name, jql string, shared bool) (*domain.Filter, error) {
	f, err := s.owned(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if f.Version != version {
		return nil, domain.ErrFilterConflict
	}
	if err := f.Edit(name, jql, shared); err != nil {
		return nil, err
	}
	return f, s.d.Filters.SaveFilter(ctx, f)
}

// DeleteFilter removes a filter (owner only).
func (s *Service) DeleteFilter(ctx context.Context, actor, id string) error {
	f, err := s.owned(ctx, actor, id)
	if err != nil {
		return err
	}
	return s.d.Filters.DeleteFilter(ctx, f.ID)
}
