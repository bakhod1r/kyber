package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

func TestVersionLifecycle(t *testing.T) {
	day := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, bad := range []string{"", "   ", strings.Repeat("v", 61)} {
		if _, err := domain.NewVersion("v-0", "KYB", bad, "", nil); !errors.Is(err, domain.ErrInvalidVersion) {
			t.Errorf("name %q: %v", bad, err)
		}
	}
	v, err := domain.NewVersion("v-1", "KYB", "  1.0  ", "First release", &day)
	if err != nil || v.Name() != "1.0" || v.Description() != "First release" || !v.ReleaseDate().Equal(day) || v.State() != domain.VersionUnreleased {
		t.Fatalf("new = %+v %v", v, err)
	}
	if v.ID() != "v-1" || v.Project() != "KYB" || v.Version() != 0 || !v.ReleasedAt().IsZero() {
		t.Fatal("identity")
	}
	if v.MarkPersisted(); v.Version() != 1 {
		t.Fatal("persisted version")
	}
	if _, err := domain.NewVersion("v-2", "KYB", "x", strings.Repeat("d", 2001), nil); !errors.Is(err, domain.ErrInvalidVersion) {
		t.Fatalf("long description: %v", err)
	}
	if err := v.Edit("1.0.0", "", nil); err != nil || v.Name() != "1.0.0" || !v.ReleaseDate().IsZero() {
		t.Fatalf("edit: %v", err)
	}
	if err := v.Edit(" ", "", nil); !errors.Is(err, domain.ErrInvalidVersion) {
		t.Fatalf("edit invalid: %v", err)
	}

	at := day.Add(time.Hour)
	if err := v.Unrelease(); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("unrelease unreleased: %v", err)
	}
	if err := v.Release(at); err != nil || v.State() != domain.VersionReleased || !v.ReleasedAt().Equal(at) || !v.ReleaseDate().Equal(at) {
		t.Fatalf("release: %v %+v", err, v)
	}
	if err := v.Release(at); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("release twice: %v", err)
	}
	if !v.CanHoldIssues() {
		t.Fatal("released versions still hold issues")
	}
	if err := v.Unrelease(); err != nil || v.State() != domain.VersionUnreleased || !v.ReleasedAt().IsZero() {
		t.Fatalf("unrelease: %v", err)
	}
	if err := v.Archive(); err != nil || v.State() != domain.VersionArchived || v.CanHoldIssues() {
		t.Fatalf("archive: %v", err)
	}
	if err := v.Archive(); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("archive twice: %v", err)
	}
	if err := v.Release(at); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("release archived: %v", err)
	}
	if err := v.Restore(); err != nil || v.State() != domain.VersionUnreleased {
		t.Fatalf("restore: %v", err)
	}
	if err := v.Restore(); !errors.Is(err, domain.ErrVersionState) {
		t.Fatalf("restore active: %v", err)
	}
	// A version released before archiving comes back released.
	_ = v.Release(at)
	_ = v.Archive()
	_ = v.Restore()
	if v.State() != domain.VersionReleased {
		t.Fatalf("restored released = %s", v.State())
	}

	r := domain.RehydrateVersion(domain.VersionSnapshot{ID: "v-9", Project: "KYB", Name: "2.0", State: domain.VersionArchived, ReleasedAt: at, Version: 3})
	if r.State() != domain.VersionArchived || r.Version() != 3 || r.Snapshot().ReleasedAt != at {
		t.Fatalf("rehydrate = %+v", r.Snapshot())
	}
}
