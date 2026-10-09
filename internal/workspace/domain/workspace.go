// Package domain is the Workspace context: a tenant (an organization's site, addressed by a
// subdomain slug) and its members. See ADR-0004.
package domain

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrInvalidSlug       = errors.New("workspace address must be 3-32 lowercase letters, digits or single hyphens, and not reserved")
	ErrSlugTaken         = errors.New("workspace address already taken")
	ErrEmptyName         = errors.New("workspace name must not be empty")
	ErrInvalidRole       = errors.New("role must be owner, admin or member")
	ErrLastOwner         = errors.New("a workspace must keep at least one owner")
	ErrWorkspaceNotFound = errors.New("workspace not found")
)

type (
	WorkspaceID string
	UserID      string
	Role        string
)

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// The default workspace holds everything in single-tenant mode (no KYBER_BASE_DOMAIN).
const (
	DefaultID   WorkspaceID = "00000000-0000-4000-8000-000000000001"
	DefaultSlug             = "default"
)

var (
	slugRe   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	reserved = []string{"www", "api", "app", "admin", "auth", "login", "static", "status", "docs", "mail", "help", "billing", DefaultSlug}
)

// ParseSlug normalises and validates a subdomain slug.
func ParseSlug(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 3 || len(s) > 32 || !slugRe.MatchString(s) || slices.Contains(reserved, s) {
		return "", ErrInvalidSlug
	}
	return s, nil
}

func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleOwner, RoleAdmin, RoleMember:
		return r, nil
	}
	return "", ErrInvalidRole
}

type Member struct {
	UserID UserID
	Role   Role
}

type Workspace struct {
	id      WorkspaceID
	slug    string
	name    string
	members map[UserID]Role
}

func NewWorkspace(id WorkspaceID, slug, name string, owner UserID) (*Workspace, error) {
	slug, err := ParseSlug(slug)
	if err != nil {
		return nil, err
	}
	if name = strings.TrimSpace(name); name == "" {
		return nil, ErrEmptyName
	}
	return &Workspace{id: id, slug: slug, name: name, members: map[UserID]Role{owner: RoleOwner}}, nil
}

// Rehydrate rebuilds a persisted workspace (repository use only).
func Rehydrate(id WorkspaceID, slug, name string, members []Member) *Workspace {
	w := &Workspace{id: id, slug: slug, name: name, members: map[UserID]Role{}}
	for _, m := range members {
		w.members[m.UserID] = m.Role
	}
	return w
}

func (w *Workspace) ID() WorkspaceID { return w.id }
func (w *Workspace) Slug() string    { return w.slug }
func (w *Workspace) Name() string    { return w.name }

func (w *Workspace) RoleOf(u UserID) (Role, bool) {
	r, ok := w.members[u]
	return r, ok
}

func (w *Workspace) IsMember(u UserID) bool { _, ok := w.members[u]; return ok }

// AddMember adds a user or changes their role; the last owner cannot be demoted.
func (w *Workspace) AddMember(u UserID, r Role) error {
	if _, err := ParseRole(string(r)); err != nil {
		return err
	}
	if w.members[u] == RoleOwner && r != RoleOwner && w.owners() == 1 {
		return ErrLastOwner
	}
	w.members[u] = r
	return nil
}

func (w *Workspace) owners() int {
	n := 0
	for _, r := range w.members {
		if r == RoleOwner {
			n++
		}
	}
	return n
}

// Members are sorted by user id for stable output.
func (w *Workspace) Members() []Member {
	out := make([]Member, 0, len(w.members))
	for u, r := range w.members {
		out = append(out, Member{UserID: u, Role: r})
	}
	slices.SortFunc(out, func(a, b Member) int { return strings.Compare(string(a.UserID), string(b.UserID)) })
	return out
}

// Repository is the persistence port.
type Repository interface {
	Create(ctx context.Context, w *Workspace) error // ErrSlugTaken
	BySlug(ctx context.Context, slug string) (*Workspace, error)
	ByID(ctx context.Context, id WorkspaceID) (*Workspace, error)
	ForUser(ctx context.Context, u UserID) ([]*Workspace, error) // sorted by slug
	SaveMembers(ctx context.Context, w *Workspace) error
	// AddMember adds a user with the role unless already a member (atomic, keeps existing roles).
	AddMember(ctx context.Context, w WorkspaceID, u UserID, r Role) error
}
