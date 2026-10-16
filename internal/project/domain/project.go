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
	ErrArchived        = errors.New("project is archived (read-only)")
)

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// DefaultWorkspace holds every project in single-tenant mode (see workspace/domain.DefaultID).
const DefaultWorkspace WorkspaceID = "00000000-0000-4000-8000-000000000001"

type (
	WorkspaceID string
	ProjectID   string
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
	id        ProjectID
	workspace WorkspaceID
	key       string
	name      string
	issueSeq  int
	members   map[UserID]Role
	archived  bool // read-only until restored (B10)
}

// NewProject creates a project whose creator becomes its first admin.
func NewProject(id ProjectID, workspace WorkspaceID, key, name string, creator UserID) (*Project, error) {
	if !keyRe.MatchString(key) {
		return nil, ErrInvalidKey
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	return &Project{id: id, workspace: workspace, key: key, name: name, members: map[UserID]Role{creator: RoleAdmin}}, nil
}

// Rehydrate rebuilds a persisted project (repository use only).
func Rehydrate(id ProjectID, workspace WorkspaceID, key, name string, issueSeq int, members []Member) *Project {
	p := &Project{id: id, workspace: workspace, key: key, name: name, issueSeq: issueSeq, members: make(map[UserID]Role, len(members))}
	for _, m := range members {
		p.members[m.UserID] = m.Role
	}
	return p
}

// RehydrateArchived rebuilds a persisted project with its archived state (repository use only).
func RehydrateArchived(id ProjectID, workspace WorkspaceID, key, name string, issueSeq int, members []Member, archived bool) *Project {
	p := Rehydrate(id, workspace, key, name, issueSeq, members)
	p.archived = archived
	return p
}

func (p *Project) ID() ProjectID { return p.id }

// Archived tells whether the project is read-only.
func (p *Project) Archived() bool { return p.archived }

// Archive makes the project read-only; admins only.
func (p *Project) Archive(by UserID) error {
	if err := p.Authorize(by, PermAdmin); err != nil {
		return err
	}
	p.archived = true
	return nil
}

// Restore makes an archived project writable again; admins only.
func (p *Project) Restore(by UserID) error {
	if r, ok := p.members[by]; !ok {
		return ErrProjectNotFound
	} else if r != RoleAdmin {
		return ErrForbidden
	}
	p.archived = false
	return nil
}

// Workspace is the tenant the project belongs to (ADR-0004).
func (p *Project) Workspace() WorkspaceID { return p.workspace }
func (p *Project) Key() string            { return p.key }
func (p *Project) Name() string           { return p.name }

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
	if p.archived && need > PermRead {
		return ErrArchived
	}
	return nil
}

// SetMember adds a member or changes their role, never leaving the project without an admin.
func (p *Project) SetMember(u UserID, r Role) error {
	if p.archived {
		return ErrArchived
	}
	if cur, ok := p.members[u]; ok && cur == RoleAdmin && r != RoleAdmin && p.adminCount() == 1 {
		return ErrLastAdmin
	}
	p.members[u] = r
	return nil
}

// RemoveMember takes a user off the project (a no-op for non-members), never its last admin.
func (p *Project) RemoveMember(u UserID) error {
	if p.members[u] == RoleAdmin && p.adminCount() == 1 {
		return ErrLastAdmin
	}
	delete(p.members, u)
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
	// InWorkspace lists every project of the request's workspace, by key.
	InWorkspace(ctx context.Context) ([]*Project, error)
}

// Template is how a new project starts out (B10): its workflow and board kind, set by the contexts that own them.
type Template string

const (
	TemplateKanban      Template = "kanban"       // To Do → In Progress → Done on a Kanban board (the default)
	TemplateScrum       Template = "scrum"        // the same workflow on a Scrum board with sprints
	TemplateBugTracking Template = "bug-tracking" // Open → In Progress → In Review → Resolved → Closed, Kanban board
)

var ErrInvalidTemplate = errors.New("template must be kanban, scrum or bug-tracking")

// ParseTemplate reads a template name; empty means Kanban.
func ParseTemplate(s string) (Template, error) {
	switch t := Template(s); t {
	case "":
		return TemplateKanban, nil
	case TemplateKanban, TemplateScrum, TemplateBugTracking:
		return t, nil
	}
	return "", ErrInvalidTemplate
}
