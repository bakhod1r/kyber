package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

// VersionSpec is a version's editable fields.
type VersionSpec struct {
	Name, Description string
	ReleaseDate       *time.Time
}

// VersionSummary is a version with its progress (only issues the actor may see are counted).
type VersionSummary struct {
	*domain.Version
	Issues, Done, Unresolved int
}

// CreateVersion adds a release to the project (project admins, like Jira's Administer Projects).
func (s *Service) CreateVersion(ctx context.Context, actor, project string, spec VersionSpec) (*domain.Version, error) {
	if err := s.d.Admins.Administer(ctx, actor, project); err != nil {
		return nil, err
	}
	v, err := domain.NewVersion(domain.VersionID(s.d.NewID()), project, spec.Name, spec.Description, spec.ReleaseDate)
	if err != nil {
		return nil, err
	}
	return v, s.d.Versions.SaveVersion(ctx, v)
}

// Versions lists the project's versions with their progress (any member).
func (s *Service) Versions(ctx context.Context, actor, project string) ([]VersionSummary, error) {
	if err := s.d.Access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	list, err := s.d.Versions.VersionsOf(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]VersionSummary, 0, len(list))
	for _, v := range list {
		issues, err := s.d.Issues.VersionIssues(ctx, actor, project, string(v.ID()))
		if err != nil {
			return nil, err
		}
		out = append(out, summarize(v, issues))
	}
	return out, nil
}

func summarize(v *domain.Version, issues []VersionIssue) VersionSummary {
	sum := VersionSummary{Version: v, Issues: len(issues)}
	for _, is := range issues {
		if is.Done {
			sum.Done++
		}
		if is.Resolution == "" {
			sum.Unresolved++
		}
	}
	return sum
}

// loadVersion finds a version the actor may act on; outsiders get ErrVersionNotFound.
func (s *Service) loadVersion(ctx context.Context, id string, allow func(project string) error) (*domain.Version, error) {
	v, err := s.d.Versions.VersionByID(ctx, domain.VersionID(id))
	if err != nil {
		return nil, err
	}
	if err := allow(v.Project()); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, err
	}
	return v, nil
}

func (s *Service) adminVersion(ctx context.Context, actor, id string) (*domain.Version, error) {
	return s.loadVersion(ctx, id, func(project string) error { return s.d.Admins.Administer(ctx, actor, project) })
}

// VersionIssues returns a version with the issues planned into it that the actor may see.
func (s *Service) VersionIssues(ctx context.Context, actor, id string) (*domain.Version, []VersionIssue, error) {
	v, err := s.loadVersion(ctx, id, func(project string) error { return s.d.Access.Authorize(ctx, actor, project, false) })
	if err != nil {
		return nil, nil, err
	}
	issues, err := s.d.Issues.VersionIssues(ctx, actor, v.Project(), id)
	return v, issues, err
}

// ReleaseNotes renders the version's release notes from the issues the actor may see.
func (s *Service) ReleaseNotes(ctx context.Context, actor, id string) (string, error) {
	v, issues, err := s.VersionIssues(ctx, actor, id)
	if err != nil {
		return "", err
	}
	notes := make([]domain.NoteIssue, len(issues))
	for i, is := range issues {
		notes[i] = domain.NoteIssue{Key: is.Key, Title: is.Title, Type: is.Type}
	}
	return domain.ReleaseNotes(v.Name(), v.Description(), notes), nil
}

// EditVersion changes a version loaded at `version` (optimistic locking).
func (s *Service) EditVersion(ctx context.Context, actor, id string, version int, spec VersionSpec) (*domain.Version, error) {
	v, err := s.adminVersion(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if v.Version() != version {
		return nil, domain.ErrVersionConflict
	}
	if err := v.Edit(spec.Name, spec.Description, spec.ReleaseDate); err != nil {
		return nil, err
	}
	return v, s.d.Versions.SaveVersion(ctx, v)
}

// otherVersion checks that `to` is another unarchived version of the same project.
func (s *Service) otherVersion(ctx context.Context, v *domain.Version, to string, unreleased bool) error {
	if to == "" {
		return nil
	}
	other, err := s.d.Versions.VersionByID(ctx, domain.VersionID(to))
	if err != nil || other.ID() == v.ID() || other.Project() != v.Project() || !other.CanHoldIssues() ||
		(unreleased && other.State() != domain.VersionUnreleased) {
		return ErrInvalidVersion
	}
	return nil
}

// ReleaseVersion ships the version. Its unresolved issues move to moveTo (another unreleased
// version), or stay when moveTo is "" — as Jira asks on release.
func (s *Service) ReleaseVersion(ctx context.Context, actor, id, moveTo string) (*domain.Version, int, error) {
	v, err := s.adminVersion(ctx, actor, id)
	if err != nil {
		return nil, 0, err
	}
	if v.State() != domain.VersionUnreleased {
		return nil, 0, domain.ErrVersionState
	}
	if err := s.otherVersion(ctx, v, moveTo, true); err != nil {
		return nil, 0, err
	}
	moved := 0
	if moveTo != "" {
		if moved, err = s.d.Issues.MoveFixVersion(ctx, actor, v.Project(), id, moveTo, true); err != nil {
			return nil, moved, err
		}
	}
	_ = v.Release(s.d.Now()) // checked above
	return v, moved, s.d.Versions.SaveVersion(ctx, v)
}

// ChangeVersionState applies a lifecycle step: unrelease, archive or restore.
func (s *Service) ChangeVersionState(ctx context.Context, actor, id string, step func(*domain.Version) error) (*domain.Version, error) {
	v, err := s.adminVersion(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if err := step(v); err != nil {
		return nil, err
	}
	return v, s.d.Versions.SaveVersion(ctx, v)
}

// DeleteVersion removes the version; its issues move to moveTo, or lose the version when "".
func (s *Service) DeleteVersion(ctx context.Context, actor, id, moveTo string) error {
	v, err := s.adminVersion(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.otherVersion(ctx, v, moveTo, false); err != nil {
		return err
	}
	if _, err := s.d.Issues.MoveFixVersion(ctx, actor, v.Project(), id, moveTo, false); err != nil {
		return err
	}
	return s.d.Versions.DeleteVersion(ctx, v.ID())
}

// CanFix implements Issue Tracking's port: issues may get unarchived versions of their project.
func (s *Service) CanFix(ctx context.Context, project, id string) error {
	v, err := s.d.Versions.VersionByID(ctx, domain.VersionID(id))
	if errors.Is(err, domain.ErrVersionNotFound) {
		return ErrInvalidVersion
	}
	if err != nil {
		return err
	}
	if v.Project() != project || !v.CanHoldIssues() {
		return ErrInvalidVersion
	}
	return nil
}
