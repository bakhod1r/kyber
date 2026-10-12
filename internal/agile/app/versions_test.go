package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/adapter/memory"
	"github.com/bakhod1r/kyber/internal/agile/app"
	"github.com/bakhod1r/kyber/internal/agile/domain"
)

func versionSetup() (*app.Service, *fakeIssues) {
	n := 0
	access := fakeAccess{"KYB": {dev: "w", viewer: "r", lead: "w"}, "OPS": {lead: "w"}}
	issues := &fakeIssues{inVersion: map[string][]app.VersionIssue{}}
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	return app.NewService(app.Deps{
		Sprints: memory.NewRepository(), Versions: memory.NewVersionRepository(), Issues: issues,
		Access: access, Admins: fakeAdmin{access},
		NewID: func() string { n++; return fmt.Sprintf("v-%d", n) },
		Now:   func() time.Time { return now },
	}), issues
}

func TestVersionsLifecycle(t *testing.T) {
	ctx := context.Background()
	s, issues := versionSetup()
	day := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	for who, want := range map[string]error{dev: app.ErrForbidden, alien: app.ErrProjectNotFound} {
		if _, err := s.CreateVersion(ctx, who, "KYB", app.VersionSpec{Name: "1.0"}); !errors.Is(err, want) {
			t.Errorf("create as %s: %v", who, err)
		}
	}
	if _, err := s.CreateVersion(ctx, lead, "KYB", app.VersionSpec{Name: " "}); !errors.Is(err, domain.ErrInvalidVersion) {
		t.Fatalf("invalid: %v", err)
	}
	v1, err := s.CreateVersion(ctx, lead, "KYB", app.VersionSpec{Name: "1.0", Description: "First", ReleaseDate: &day})
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := s.CreateVersion(ctx, lead, "KYB", app.VersionSpec{Name: "1.1"})
	ops, _ := s.CreateVersion(ctx, lead, "OPS", app.VersionSpec{Name: "x"})
	id1, id2 := string(v1.ID()), string(v2.ID())
	issues.inVersion[id1] = []app.VersionIssue{
		{Key: "KYB-1", Title: "Login", Type: "story", Done: true, Resolution: "done"},
		{Key: "KYB-2", Title: "Crash", Type: "bug"},
	}

	list, err := s.Versions(ctx, viewer, "KYB")
	if err != nil || len(list) != 2 || list[0].Name() != "1.0" || list[0].Issues != 2 || list[0].Done != 1 || list[0].Unresolved != 1 {
		t.Fatalf("versions = %+v %v", list, err)
	}
	if _, err := s.Versions(ctx, alien, "KYB"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("outsider list: %v", err)
	}
	notes, err := s.ReleaseNotes(ctx, viewer, id1)
	if err != nil || !strings.Contains(notes, "## Bug fixes\n\n- [KYB-2] Crash") || !strings.Contains(notes, "First") {
		t.Fatalf("notes = %q %v", notes, err)
	}
	if _, err := s.ReleaseNotes(ctx, alien, id1); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("outsider notes: %v", err)
	}

	// Edit with optimistic locking; members may not.
	if _, err := s.EditVersion(ctx, dev, id2, 1, app.VersionSpec{Name: "x"}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member edit: %v", err)
	}
	if _, err := s.EditVersion(ctx, lead, id2, 9, app.VersionSpec{Name: "x"}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	if _, err := s.EditVersion(ctx, lead, id2, 1, app.VersionSpec{Name: ""}); !errors.Is(err, domain.ErrInvalidVersion) {
		t.Fatalf("invalid edit: %v", err)
	}
	if v, err := s.EditVersion(ctx, lead, id2, 1, app.VersionSpec{Name: "2.0"}); err != nil || v.Name() != "2.0" {
		t.Fatalf("edit: %v", err)
	}
	if _, err := s.EditVersion(ctx, lead, "v-404", 1, app.VersionSpec{Name: "x"}); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("missing: %v", err)
	}

	// Release: unresolved issues move only to another unreleased version of the project.
	for _, bad := range []string{id1, string(ops.ID()), "v-404"} {
		if _, _, err := s.ReleaseVersion(ctx, lead, id1, bad); !errors.Is(err, app.ErrInvalidVersion) {
			t.Errorf("release moving to %s: %v", bad, err)
		}
	}
	issues.moveErr = errors.New("db down")
	if _, _, err := s.ReleaseVersion(ctx, lead, id1, id2); err == nil {
		t.Fatal("move error swallowed")
	}
	issues.moveErr = nil
	v, moved, err := s.ReleaseVersion(ctx, lead, id1, id2)
	if err != nil || moved != 1 || v.State() != domain.VersionReleased || len(issues.inVersion[id2]) != 1 {
		t.Fatalf("release = %v moved %d %v", v, moved, err)
	}
	if _, _, err := s.ReleaseVersion(ctx, lead, id1, ""); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("release twice: %v", err)
	}
	if _, _, err := s.ReleaseVersion(ctx, dev, id1, ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member release: %v", err)
	}
	if _, _, err := s.ReleaseVersion(ctx, lead, id2, id1); !errors.Is(err, app.ErrInvalidVersion) {
		t.Fatalf("moving into a released version: %v", err)
	}

	// Lifecycle steps.
	if v, err := s.ChangeVersionState(ctx, lead, id1, (*domain.Version).Archive); err != nil || v.State() != domain.VersionArchived {
		t.Fatalf("archive: %v", err)
	}
	if err := s.CanFix(ctx, "KYB", id1); !errors.Is(err, app.ErrInvalidVersion) {
		t.Fatalf("archived version takes issues: %v", err)
	}
	if _, err := s.ChangeVersionState(ctx, lead, id1, (*domain.Version).Archive); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("archive twice: %v", err)
	}
	if _, err := s.ChangeVersionState(ctx, dev, id1, (*domain.Version).Restore); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member restore: %v", err)
	}

	// CanFix.
	if err := s.CanFix(ctx, "KYB", id2); err != nil {
		t.Fatalf("can fix: %v", err)
	}
	for project, id := range map[string]string{"OPS": id2, "KYB": "v-404"} {
		if err := s.CanFix(ctx, project, id); !errors.Is(err, app.ErrInvalidVersion) {
			t.Errorf("CanFix %s/%s: %v", project, id, err)
		}
	}

	// Delete moves every issue (or drops the version from them).
	if err := s.DeleteVersion(ctx, lead, id2, string(ops.ID())); !errors.Is(err, app.ErrInvalidVersion) {
		t.Fatalf("delete moving to another project: %v", err)
	}
	if err := s.DeleteVersion(ctx, dev, id2, ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("member delete: %v", err)
	}
	issues.moveErr = errors.New("db down")
	if err := s.DeleteVersion(ctx, lead, id2, ""); err == nil {
		t.Fatal("move error swallowed")
	}
	issues.moveErr = nil
	if err := s.DeleteVersion(ctx, lead, id2, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteVersion(ctx, lead, id2, ""); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if !strings.Contains(strings.Join(issues.calls, "|"), "move KYB/"+id2+"→ unresolved=false") {
		t.Fatalf("calls = %v", issues.calls)
	}

	// Listing fails when Issue Tracking does.
	issues.err = errors.New("db down")
	if _, err := s.Versions(ctx, viewer, "KYB"); err == nil {
		t.Fatal("issue error swallowed")
	}
}
