// Package domain is the Administration context's read model: what a workspace's administrators see
// of its members, projects and stuck events. It owns no data; other contexts are read through ACLs.
package domain

import (
	"errors"
	"time"
)

var (
	ErrForbidden = errors.New("only workspace administrators may do this")
	// ErrSingleTenant: the default workspace stores no roles (its administrators are configured).
	ErrSingleTenant = errors.New("roles are configured for the default workspace (KYBER_ADMIN_EMAILS)")
	ErrLastOwner    = errors.New("a workspace must keep at least one owner")
	ErrNotMember    = errors.New("not a member of the workspace")
	ErrInvalidRole  = errors.New("role must be owner, admin or member")
	// ErrNoDeadLetter: no stuck event with that id in this workspace.
	ErrNoDeadLetter = errors.New("no such stuck event")
)

// Member is someone in the workspace with their workspace role.
type Member struct {
	ID, Name, Email string
	Role            string // owner, admin or member
}

// Project is a project of the workspace.
type Project struct {
	Key, Name string
	Members   int
}

// DeadLetter is an event whose handlers kept failing (ADR-0007).
type DeadLetter struct {
	ID        int64
	Event     string
	Actor     string
	At        time.Time
	Attempts  int
	LastError string
}
