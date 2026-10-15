// Package domain maps identity-provider groups to Kyber roles (S106, ADR-0015 amendment): a site
// admin maps a group to a workspace or project role; every SSO login grants the roles the user's
// groups map to and revokes the ones SSO granted earlier that no longer apply. Roles given by hand
// are never touched. Pure: no I/O.
package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidMapping  = errors.New("invalid group mapping")
	ErrMappingNotFound = errors.New("group mapping not found")
)

type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeProject   Scope = "project"
)

// Target is a workspace, or a project (by key) in a workspace.
type Target struct {
	Scope     Scope
	Workspace string
	Project   string // project key; "" for a workspace
}

func (t Target) String() string {
	if t.Scope == ScopeProject {
		return t.Workspace + "/" + t.Project
	}
	return t.Workspace
}

type MappingID string

// Mapping grants Role on Target to members of Group.
type Mapping struct {
	ID        MappingID
	Group     string
	Target    Target
	Role      string
	CreatedBy string
	CreatedAt time.Time
}

// roles per scope, by rank. Workspace owners stay a human decision.
var roles = map[Scope]map[string]int{
	ScopeWorkspace: {"member": 1, "admin": 2},
	ScopeProject:   {"viewer": 1, "member": 2, "admin": 3},
}

func NewMapping(id MappingID, group string, t Target, role, by string, at time.Time) (*Mapping, error) {
	group = strings.TrimSpace(group)
	if group == "" || utf8.RuneCountInString(group) > 200 {
		return nil, fmt.Errorf("%w: the group must be 1-200 characters", ErrInvalidMapping)
	}
	ranks, ok := roles[t.Scope]
	if !ok || t.Workspace == "" {
		return nil, fmt.Errorf("%w: the target must be a workspace or a project", ErrInvalidMapping)
	}
	t.Project = strings.ToUpper(strings.TrimSpace(t.Project))
	if (t.Scope == ScopeProject) != (t.Project != "") {
		return nil, fmt.Errorf("%w: a project target needs a project key", ErrInvalidMapping)
	}
	if _, ok := ranks[role]; !ok {
		return nil, fmt.Errorf("%w: role %q is not grantable on a %s", ErrInvalidMapping, role, t.Scope)
	}
	return &Mapping{ID: id, Group: group, Target: t, Role: role, CreatedBy: by, CreatedAt: at}, nil
}

// Grant is a role SSO gave a user (so SSO may take it back).
type Grant struct {
	User   string
	Target Target
	Role   string
}

// Plan compares what the user's groups map to with what SSO granted before: set lists roles to
// grant or change (the highest mapped role per target wins), revoke the earlier grants no group
// supports any more. Groups match case-insensitively.
func Plan(user string, mappings []Mapping, groups []string, held []Grant) (set, revoke []Grant) {
	member := map[string]bool{}
	for _, g := range groups {
		member[strings.ToLower(strings.TrimSpace(g))] = true
	}
	want := map[Target]string{}
	for _, m := range mappings {
		if !member[strings.ToLower(m.Group)] {
			continue
		}
		if cur, ok := want[m.Target]; !ok || roles[m.Target.Scope][m.Role] > roles[m.Target.Scope][cur] {
			want[m.Target] = m.Role
		}
	}
	had := map[Target]string{}
	for _, g := range held {
		had[g.Target] = g.Role
		if _, ok := want[g.Target]; !ok {
			revoke = append(revoke, g)
		}
	}
	for t, r := range want {
		if had[t] != r {
			set = append(set, Grant{User: user, Target: t, Role: r})
		}
	}
	sort.Slice(set, func(i, j int) bool { return set[i].Target.String() < set[j].Target.String() })
	return set, revoke
}

// Mappings stores the installation's group mappings (site-wide; targets name their workspace).
type Mappings interface {
	All(ctx context.Context) ([]Mapping, error)
	Create(ctx context.Context, m *Mapping) error
	Delete(ctx context.Context, id MappingID) error // ErrMappingNotFound
}

// Grants remembers which roles SSO granted to whom.
type Grants interface {
	Of(ctx context.Context, user string) ([]Grant, error)
	Save(ctx context.Context, g Grant) error // creates or replaces the user's grant on that target
	Remove(ctx context.Context, user string, t Target) error
}
