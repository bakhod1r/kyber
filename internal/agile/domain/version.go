package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidVersion  = errors.New("invalid version")
	ErrVersionNotFound = errors.New("version not found")
	ErrVersionState    = errors.New("the version is not in a state that allows this")
	ErrVersionConflict = errors.New("version was changed concurrently")
	ErrVersionTaken    = errors.New("a version with this name already exists in the project")
)

type (
	VersionID    string
	VersionState string
)

const (
	VersionUnreleased VersionState = "unreleased"
	VersionReleased   VersionState = "released"
	VersionArchived   VersionState = "archived"
)

const (
	maxVersionName        = 60
	maxVersionDescription = 2000
)

// Version is a project release (Jira's fix version): issues are planned into it and it is
// released when they ship. Archived versions are hidden from pickers.
type Version struct {
	id          VersionID
	project     string
	name        string
	description string
	releaseDate time.Time // planned date until released, then the release moment; zero = none
	state       VersionState
	releasedAt  time.Time
	version     int
	events      []Event
}

// VersionShipped: the version was released; Issue Tracking moves its unresolved issues to
// MoveUnresolvedTo (when set). By is who released it.
type VersionShipped struct {
	ID               VersionID `json:"id"`
	Project          string    `json:"project"`
	MoveUnresolvedTo VersionID `json:"move_unresolved_to"`
	By               string    `json:"by"`
}

// VersionRemoved: the version is gone; Issue Tracking moves its issues to MoveTo, or drops it.
type VersionRemoved struct {
	ID      VersionID `json:"id"`
	Project string    `json:"project"`
	MoveTo  VersionID `json:"move_to"`
	By      string    `json:"by"`
}

func (VersionShipped) EventName() string { return "version.released" }
func (VersionRemoved) EventName() string { return "version.deleted" }

// VersionSnapshot is the persisted state of a version.
type VersionSnapshot struct {
	ID          VersionID
	Project     string
	Name        string
	Description string
	ReleaseDate time.Time
	State       VersionState
	ReleasedAt  time.Time
	Version     int
}

func NewVersion(id VersionID, project, name, description string, releaseDate *time.Time) (*Version, error) {
	v := &Version{id: id, project: project, state: VersionUnreleased}
	if err := v.Edit(name, description, releaseDate); err != nil {
		return nil, err
	}
	return v, nil
}

func RehydrateVersion(s VersionSnapshot) *Version {
	return &Version{id: s.ID, project: s.Project, name: s.Name, description: s.Description, releaseDate: s.ReleaseDate,
		state: s.State, releasedAt: s.ReleasedAt, version: s.Version}
}

func (v *Version) Snapshot() VersionSnapshot {
	return VersionSnapshot{ID: v.id, Project: v.project, Name: v.name, Description: v.description, ReleaseDate: v.releaseDate,
		State: v.state, ReleasedAt: v.releasedAt, Version: v.version}
}

func (v *Version) ID() VersionID          { return v.id }
func (v *Version) Project() string        { return v.project }
func (v *Version) Name() string           { return v.name }
func (v *Version) Description() string    { return v.description }
func (v *Version) ReleaseDate() time.Time { return v.releaseDate }
func (v *Version) State() VersionState    { return v.state }
func (v *Version) ReleasedAt() time.Time  { return v.releasedAt }
func (v *Version) Version() int           { return v.version }
func (v *Version) CanHoldIssues() bool    { return v.state != VersionArchived }
func (v *Version) MarkPersisted()         { v.version++ }

// Edit changes the name, description and planned release date (nil = none).
func (v *Version) Edit(name, description string, releaseDate *time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxVersionName {
		return fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidVersion, maxVersionName)
	}
	if utf8.RuneCountInString(description) > maxVersionDescription {
		return fmt.Errorf("%w: description is longer than %d characters", ErrInvalidVersion, maxVersionDescription)
	}
	v.name, v.description, v.releaseDate = name, description, time.Time{}
	if releaseDate != nil {
		v.releaseDate = releaseDate.UTC()
	}
	return nil
}

// Release marks an unreleased version as shipped at `at`, by `by`; its unresolved issues are to
// move on to moveUnresolvedTo ("" = they stay).
func (v *Version) Release(at time.Time, by string, moveUnresolvedTo VersionID) error {
	if v.state != VersionUnreleased {
		return ErrVersionState
	}
	if moveUnresolvedTo == v.id {
		return fmt.Errorf("%w: unresolved issues cannot move to the version being released", ErrInvalidVersion)
	}
	v.state, v.releasedAt, v.releaseDate = VersionReleased, at, at
	v.events = append(v.events, VersionShipped{ID: v.id, Project: v.project, MoveUnresolvedTo: moveUnresolvedTo, By: by})
	return nil
}

// Delete records the version's removal; its issues are to move to moveTo ("" = drop it).
func (v *Version) Delete(by string, moveTo VersionID) error {
	if moveTo == v.id {
		return fmt.Errorf("%w: issues cannot move to the version being deleted", ErrInvalidVersion)
	}
	v.events = append(v.events, VersionRemoved{ID: v.id, Project: v.project, MoveTo: moveTo, By: by})
	return nil
}

// PullEvents returns and clears the recorded domain events.
func (v *Version) PullEvents() []Event {
	ev := v.events
	v.events = nil
	return ev
}

// Unrelease reopens a released version.
func (v *Version) Unrelease() error {
	if v.state != VersionReleased {
		return ErrVersionState
	}
	v.state, v.releasedAt = VersionUnreleased, time.Time{}
	return nil
}

// Archive hides the version; it keeps its issues but takes no new ones.
func (v *Version) Archive() error {
	if v.state == VersionArchived {
		return ErrVersionState
	}
	v.state = VersionArchived
	return nil
}

// Restore brings an archived version back as released or unreleased.
func (v *Version) Restore() error {
	if v.state != VersionArchived {
		return ErrVersionState
	}
	v.state = VersionUnreleased
	if !v.releasedAt.IsZero() {
		v.state = VersionReleased
	}
	return nil
}

// VersionRepository stores versions in the workspace of ctx. Saving and deleting write the
// version's pulled domain events to the outbox in the same transaction.
type VersionRepository interface {
	// SaveVersion inserts (version 0) or updates with optimistic locking; ErrVersionTaken on a duplicate name.
	SaveVersion(ctx context.Context, v *Version) error
	VersionByID(ctx context.Context, id VersionID) (*Version, error)
	// VersionsOf lists the project's versions: unreleased first by release date, then by name.
	VersionsOf(ctx context.Context, project string) ([]*Version, error)
	// DeleteVersion removes the version (call Delete on it first, for its VersionRemoved event).
	DeleteVersion(ctx context.Context, v *Version) error
}
