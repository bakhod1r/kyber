// Package domain is the Access context: permission schemes (RBAC) with attribute rules
// (ABAC: reporter/assignee holders, issue security levels, read-only statuses). Pure, no I/O.
// See ADR-0003.
package domain

import (
	"errors"
	"fmt"
	"slices"

	guard "github.com/bakhod1r/guard/access/domain"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("permission denied")
	ErrInvalidLevel = errors.New("security level needs a unique id")
)

type (
	UserID     string
	RoleID     string
	Permission string
)

const (
	RoleAdmin  RoleID = "admin"
	RoleMember RoleID = "member"
	RoleViewer RoleID = "viewer"
)

const (
	BrowseProjects     Permission = "BROWSE_PROJECTS"
	CreateIssues       Permission = "CREATE_ISSUES"
	EditIssues         Permission = "EDIT_ISSUES"
	DeleteIssues       Permission = "DELETE_ISSUES"
	AssignIssues       Permission = "ASSIGN_ISSUES"
	AssignableUser     Permission = "ASSIGNABLE_USER"
	TransitionIssues   Permission = "TRANSITION_ISSUES"
	CloseIssues        Permission = "CLOSE_ISSUES"
	AddComments        Permission = "ADD_COMMENTS"
	EditAllComments    Permission = "EDIT_ALL_COMMENTS"
	DeleteAllComments  Permission = "DELETE_ALL_COMMENTS"
	ManageSprints      Permission = "MANAGE_SPRINTS"
	AdministerProjects Permission = "ADMINISTER_PROJECTS"
)

var permissions = []Permission{BrowseProjects, CreateIssues, EditIssues, DeleteIssues, AssignIssues, AssignableUser,
	TransitionIssues, CloseIssues, AddComments, EditAllComments, DeleteAllComments, ManageSprints, AdministerProjects}

// Permissions lists every permission in display order.
func Permissions() []Permission { return slices.Clone(permissions) }

// editing permissions are denied while an issue is in a read-only status.
var editing = []Permission{EditIssues, DeleteIssues, AssignIssues, AddComments, EditAllComments, DeleteAllComments}

type holderKind int

const (
	holderRole holderKind = iota + 1
	holderUser
	holderReporter
	holderAssignee
)

// Grant is who holds a permission: a project role, a user, or an issue attribute.
type Grant struct {
	kind  holderKind
	value string
}

func Role(r RoleID) Grant { return Grant{holderRole, string(r)} }
func User(u UserID) Grant { return Grant{holderUser, string(u)} }
func Reporter() Grant     { return Grant{kind: holderReporter} }
func Assignee() Grant     { return Grant{kind: holderAssignee} }
func (g Grant) String() string {
	switch g.kind {
	case holderRole:
		return "role " + g.value
	case holderUser:
		return "user " + g.value
	case holderReporter:
		return "own issue (reporter)"
	default:
		return "assignee"
	}
}

// Subject is the acting user with their roles in the project.
type Subject struct {
	User  UserID
	Roles []RoleID
}

// IssueAttrs are the attributes ABAC rules look at; nil means a project-level check.
type IssueAttrs struct {
	Reporter, Assignee UserID
	Status             string
	SecurityLevel      string // "" = visible to everyone who can browse
}

// SecurityLevel restricts who can see an issue (issue-level security).
type SecurityLevel struct {
	ID, Name string
	Grants   []Grant
}

// Scheme is a permission scheme shared by projects.
type Scheme struct {
	grants map[Permission][]Grant
	levels map[string]SecurityLevel
	locked []string
}

// DefaultScheme reproduces viewer < member < admin, plus "edit/delete own issue" for reporters.
func DefaultScheme() *Scheme {
	s := &Scheme{grants: map[Permission][]Grant{}, levels: map[string]SecurityLevel{}}
	viewer, member, admin := Role(RoleViewer), Role(RoleMember), Role(RoleAdmin)
	s.grants[BrowseProjects] = []Grant{viewer, member, admin}
	for _, p := range []Permission{CreateIssues, AssignIssues, AssignableUser, TransitionIssues, CloseIssues, AddComments, ManageSprints} {
		s.grants[p] = []Grant{member, admin}
	}
	s.grants[EditIssues] = []Grant{member, admin, Reporter()}
	s.grants[DeleteIssues] = []Grant{admin, Reporter()}
	for _, p := range []Permission{EditAllComments, DeleteAllComments, AdministerProjects} {
		s.grants[p] = []Grant{admin}
	}
	return s
}

// Grant adds a holder to a permission (idempotent).
func (s *Scheme) Grant(p Permission, g Grant) {
	if !slices.Contains(s.grants[p], g) {
		s.grants[p] = append(s.grants[p], g)
	}
}

// Revoke removes a holder from a permission.
func (s *Scheme) Revoke(p Permission, g Grant) {
	s.grants[p] = slices.DeleteFunc(s.grants[p], func(x Grant) bool { return x == g })
}

// Grants lists a permission's holders.
func (s *Scheme) Grants(p Permission) []Grant { return slices.Clone(s.grants[p]) }

// AddLevel defines an issue security level.
func (s *Scheme) AddLevel(l SecurityLevel) error {
	if _, dup := s.levels[l.ID]; l.ID == "" || dup {
		return fmt.Errorf("%w: %q", ErrInvalidLevel, l.ID)
	}
	s.levels[l.ID] = l
	return nil
}

// LockStatus makes issues in the status read-only (they can still be browsed and transitioned).
func (s *Scheme) LockStatus(status string) { s.locked = append(s.locked, status) }

// Decision is an access decision with a human-readable reason (for audit and 403 details).
type Decision struct {
	Allowed bool
	Reason  string
	perm    Permission
	hidden  bool // the subject may not know the resource exists
}

// Err maps a denial to ErrNotFound (resource hidden) or ErrForbidden.
func (d Decision) Err() error {
	switch {
	case d.Allowed:
		return nil
	case d.hidden:
		return fmt.Errorf("%w: %s", ErrNotFound, d.Reason)
	}
	return fmt.Errorf("%w: %s requires %s", ErrForbidden, d.Reason, d.perm)
}

// Decide answers whether sub may exercise p, on the issue when is != nil. Default deny.
// The scheme is compiled into guard ABAC policies; guard's engine makes the decision.
func (s *Scheme) Decide(sub Subject, p Permission, is *IssueAttrs) Decision {
	deny := func(hidden bool, reason string) Decision { return Decision{Reason: reason, perm: p, hidden: hidden} }
	switch {
	case sub.User == "":
		return deny(true, "anonymous")
	case !slices.Contains(permissions, p):
		return deny(false, "unknown permission")
	}
	policies := s.policies()
	browse := guard.Decide(request(sub, BrowseProjects, nil), policies)
	if !browse.Allowed {
		return deny(true, "cannot browse project")
	}
	if is != nil && is.SecurityLevel != "" {
		if v := guard.Decide(request(sub, levelAction, is), policies); !v.Allowed {
			return deny(true, reason(v))
		}
	}
	if p == BrowseProjects {
		return Decision{Allowed: true, Reason: reason(browse), perm: p}
	}
	d := guard.Decide(request(sub, p, is), policies)
	return Decision{Allowed: d.Allowed, Reason: reason(d), perm: p}
}
