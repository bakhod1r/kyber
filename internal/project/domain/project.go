package domain

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrInvalidKey      = errors.New("project key must be 2-10 uppercase letters/digits starting with a letter")
	ErrEmptyName       = errors.New("project name must not be empty")
	ErrProjectNotFound = errors.New("project not found")
	ErrKeyTaken        = errors.New("project key already taken")
	ErrInvalidRole     = errors.New("role must be one of admin, member, viewer")
	ErrForbidden       = errors.New("insufficient project role")
	ErrLastAdmin       = errors.New("project must keep at least one admin")
)

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

type (
	ProjectID string
	// UserID is the Identity context's user identifier (published language).
	UserID string
	Role   string
)

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

func ParseRole(s string) (Role, error) {
	switch r := Role(s); r {
	case RoleAdmin, RoleMember, RoleViewer:
		return r, nil
	}
	return "", ErrInvalidRole
}

// Permission is what an action needs; roles are ordered viewer < member < admin.
type Permission int

const (
	PermRead Permission = iota + 1
	PermWrite
	PermAdmin
)

func (r Role) grants() Permission {
	switch r {
	case RoleAdmin:
		return PermAdmin
	case RoleMember:
		return PermWrite
	case RoleViewer:
		return PermRead
	}
	return 0
}

type Member struct {
	UserID UserID
	Role   Role
}

// Project is the aggregate root of the Project context. It owns membership and the issue sequence.
type Project struct {
	id       ProjectID
	key      string
	name     string
	issueSeq int
	members  map[UserID]Role
}

// NewProject creates a project whose creator becomes its first admin.
func NewProject(id ProjectID, key, name string, creator UserID) (*Project, error) {
	if !keyRe.MatchString(key) {
		return nil, ErrInvalidKey
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	return &Project{id: id, key: key, name: name, members: map[UserID]Role{creator: RoleAdmin}}, nil
}

// Rehydrate rebuilds a persisted project (repository use only).
func Rehydrate(id ProjectID, key, name string, issueSeq int, members []Member) *Project {
	p := &Project{id: id, key: key, name: name, issueSeq: issueSeq, members: make(map[UserID]Role, len(members))}
	for _, m := range members {
		p.members[m.UserID] = m.Role
	}
	return p
}

func (p *Project) ID() ProjectID { return p.id }
func (p *Project) Key() string   { return p.key }
func (p *Project) Name() string  { return p.name }

// IssueSeq is the last allocated issue number.
func (p *Project) IssueSeq() int { return p.issueSeq }

// NextIssueNumber advances and returns the project's issue sequence.
func (p *Project) NextIssueNumber() int {
	p.issueSeq++
	return p.issueSeq
}

func (p *Project) RoleOf(u UserID) (Role, bool) {
	r, ok := p.members[u]
	return r, ok
}

// Authorize hides the project from non-members (ErrProjectNotFound) and
// rejects members whose role is too low (ErrForbidden).
func (p *Project) Authorize(u UserID, need Permission) error {
	r, ok := p.members[u]
	if !ok {
		return ErrProjectNotFound
	}
	if r.grants() < need {
		return ErrForbidden
	}
	return nil
}

// SetMember adds a member or changes their role, never leaving the project without an admin.
func (p *Project) SetMember(u UserID, r Role) error {
	if cur, ok := p.members[u]; ok && cur == RoleAdmin && r != RoleAdmin && p.adminCount() == 1 {
		return ErrLastAdmin
	}
	p.members[u] = r
	return nil
}

// Members returns members sorted by user ID.
func (p *Project) Members() []Member {
	out := make([]Member, 0, len(p.members))
	for u, r := range p.members {
		out = append(out, Member{UserID: u, Role: r})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func (p *Project) adminCount() int {
	n := 0
	for _, r := range p.members {
		if r == RoleAdmin {
			n++
		}
	}
	return n
}

// Repository is the persistence port for the Project aggregate.
type Repository interface {
	Create(ctx context.Context, p *Project) error // ErrKeyTaken
	// Update loads, mutates and stores atomically (row lock in Postgres).
	Update(ctx context.Context, key string, fn func(*Project) error) error
	ByKey(ctx context.Context, key string) (*Project, error) // ErrProjectNotFound
	ListForUser(ctx context.Context, u UserID) ([]*Project, error)
}
