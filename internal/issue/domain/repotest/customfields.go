package repotest

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// CustomFieldFactory returns an empty repository; projects KYB and OPS exist in the default
// workspace and KYB in OtherWorkspace.
type CustomFieldFactory func(t *testing.T) domain.CustomFieldRepository

func RunCustomFields(t *testing.T, newRepo CustomFieldFactory) {
	ctx := tenant.With(context.Background(), tenant.Default)
	repo := newRepo(t)
	mk := func(n int, project, name string, typ domain.FieldType, opts ...string) *domain.CustomField {
		t.Helper()
		f, err := domain.NewCustomField(domain.FieldID(uuidFor(300+n)), project, name, typ, opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveField(ctx, f); err != nil || f.Version() != 1 {
			t.Fatalf("save %s: %v", name, err)
		}
		return f
	}
	tier := mk(1, "KYB", "Tier", domain.FieldSelect, "Gold", "Silver")
	mk(2, "KYB", "Customer", domain.FieldText)
	mk(3, "OPS", "Tier", domain.FieldNumber)
	list, err := repo.FieldsOf(ctx, "KYB")
	names := []string{}
	for _, f := range list {
		names = append(names, f.Name())
	}
	if err != nil || strings.Join(names, ",") != "Tier,Customer" {
		t.Fatalf("fields = %v %v (creation order)", names, err)
	}
	got, err := repo.FieldByID(ctx, tier.ID())
	if err != nil || got.Type() != domain.FieldSelect || strings.Join(got.Options(), ",") != "Gold,Silver" || got.Project() != "KYB" {
		t.Fatalf("by id = %+v %v", got.Snapshot(), err)
	}
	stale, _ := repo.FieldByID(ctx, tier.ID())
	_ = got.Edit("Level", []string{"Gold"})
	if err := repo.SaveField(ctx, got); err != nil || got.Version() != 2 {
		t.Fatalf("update: %v", err)
	}
	if err := repo.SaveField(ctx, stale); !errors.Is(err, domain.ErrFieldConflict) {
		t.Fatalf("stale: %v", err)
	}
	dup, _ := domain.NewCustomField(domain.FieldID(uuidFor(304)), "KYB", "customer", domain.FieldText, nil)
	if err := repo.SaveField(ctx, dup); !errors.Is(err, domain.ErrFieldConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
	other := tenant.With(context.Background(), OtherWorkspace)
	if _, err := repo.FieldByID(other, tier.ID()); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("other workspace: %v", err)
	}
	if err := repo.DeleteField(other, tier.ID()); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("delete from other workspace: %v", err)
	}
	if err := repo.DeleteField(ctx, tier.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FieldByID(ctx, tier.ID()); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := repo.DeleteField(ctx, tier.ID()); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	if _, err := repo.FieldByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}
