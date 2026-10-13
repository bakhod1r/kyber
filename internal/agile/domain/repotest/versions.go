package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Version IDs adapters must accept (UUIDs for Postgres).
const (
	VersionA = "90000000-0000-4000-8000-000000000001"
	VersionB = "90000000-0000-4000-8000-000000000002"
	VersionC = "90000000-0000-4000-8000-000000000003"
	VersionD = "90000000-0000-4000-8000-000000000004"
)

// VersionFactory returns an empty version repository and a reader of the stored outbox event
// names; projects KYB and OPS exist in the default workspace and KYB in OtherWorkspace.
type VersionFactory func(t *testing.T) (domain.VersionRepository, func() []string)

func RunVersions(t *testing.T, newRepo VersionFactory) {
	ctx := tenant.With(context.Background(), tenant.Default)
	day := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	mk := func(t *testing.T, id, project, name string, date *time.Time) *domain.Version {
		t.Helper()
		v, err := domain.NewVersion(domain.VersionID(id), project, name, "notes for "+name, date)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	t.Run("save, load, list in order", func(t *testing.T) {
		repo, _ := newRepo(t)
		later := day.Add(24 * time.Hour)
		for _, v := range []*domain.Version{mk(t, VersionA, "KYB", "2.0", &later), mk(t, VersionB, "KYB", "1.0", &day),
			mk(t, VersionC, "KYB", "Backlog ideas", nil), mk(t, VersionD, "OPS", "1.0", nil)} {
			if err := repo.SaveVersion(ctx, v); err != nil || v.Version() != 1 {
				t.Fatalf("save %s: %v", v.Name(), err)
			}
		}
		got, err := repo.VersionByID(ctx, VersionB)
		if err != nil || got.Name() != "1.0" || got.Description() != "notes for 1.0" || !got.ReleaseDate().Equal(day) ||
			got.State() != domain.VersionUnreleased || got.Project() != "KYB" || got.Version() != 1 {
			t.Fatalf("loaded = %+v %v", got.Snapshot(), err)
		}
		list, _ := repo.VersionsOf(ctx, "KYB")
		names := ""
		for _, v := range list {
			names += v.Name() + "|"
		}
		if names != "1.0|2.0|Backlog ideas|" {
			t.Fatalf("order = %s", names)
		}
	})

	t.Run("update with optimistic locking; names unique per project", func(t *testing.T) {
		repo, outbox := newRepo(t)
		v := mk(t, VersionA, "KYB", "1.0", nil)
		_ = repo.SaveVersion(ctx, v)
		stale, _ := repo.VersionByID(ctx, VersionA)
		at := day.Add(time.Hour)
		_ = v.Release(at, "u-1", "")
		if err := repo.SaveVersion(ctx, v); err != nil || v.Version() != 2 {
			t.Fatalf("update: %v", err)
		}
		if names := outbox(); len(names) != 1 || names[0] != "version.released" {
			t.Fatalf("outbox = %v (the event is saved with the version)", names)
		}
		got, _ := repo.VersionByID(ctx, VersionA)
		if got.State() != domain.VersionReleased || !got.ReleasedAt().Equal(at) {
			t.Fatalf("released = %+v", got.Snapshot())
		}
		_ = stale.Archive()
		if err := repo.SaveVersion(ctx, stale); !errors.Is(err, domain.ErrVersionConflict) {
			t.Fatalf("stale save: %v", err)
		}
		dup := mk(t, VersionB, "KYB", "1.0", nil)
		if err := repo.SaveVersion(ctx, dup); !errors.Is(err, domain.ErrVersionTaken) {
			t.Fatalf("duplicate name: %v", err)
		}
		_ = repo.SaveVersion(ctx, mk(t, VersionC, "KYB", "1.1", nil))
		renamed, _ := repo.VersionByID(ctx, VersionC)
		_ = renamed.Edit("1.0", "", nil)
		if err := repo.SaveVersion(ctx, renamed); !errors.Is(err, domain.ErrVersionTaken) {
			t.Fatalf("rename onto another: %v", err)
		}
		if err := repo.SaveVersion(ctx, mk(t, VersionD, "OPS", "1.0", nil)); err != nil {
			t.Fatalf("same name in another project: %v", err)
		}
	})

	t.Run("delete and workspace isolation", func(t *testing.T) {
		repo, outbox := newRepo(t)
		a := mk(t, VersionA, "KYB", "1.0", nil)
		_ = repo.SaveVersion(ctx, a)
		other := tenant.With(context.Background(), OtherWorkspace)
		if _, err := repo.VersionByID(other, VersionA); !errors.Is(err, domain.ErrVersionNotFound) {
			t.Fatalf("other workspace sees it: %v", err)
		}
		if list, _ := repo.VersionsOf(other, "KYB"); len(list) != 0 {
			t.Fatal("other workspace lists it")
		}
		if err := repo.DeleteVersion(other, a); !errors.Is(err, domain.ErrVersionNotFound) {
			t.Fatalf("delete from other workspace: %v", err)
		}
		if err := repo.SaveVersion(other, mk(t, VersionB, "KYB", "1.0", nil)); err != nil {
			t.Fatalf("same name in another workspace: %v", err)
		}
		_ = a.Delete("u-1", "")
		if err := repo.DeleteVersion(ctx, a); err != nil {
			t.Fatal(err)
		}
		if names := outbox(); len(names) != 1 || names[0] != "version.deleted" {
			t.Fatalf("outbox = %v", names)
		}
		if _, err := repo.VersionByID(ctx, VersionA); !errors.Is(err, domain.ErrVersionNotFound) {
			t.Fatalf("deleted: %v", err)
		}
		if err := repo.DeleteVersion(ctx, a); !errors.Is(err, domain.ErrVersionNotFound) {
			t.Fatalf("delete twice: %v", err)
		}
		if _, err := repo.VersionByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrVersionNotFound) {
			t.Fatalf("bad id: %v", err)
		}
	})
}
