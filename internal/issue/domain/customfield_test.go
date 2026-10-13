package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestCustomFieldDefinition(t *testing.T) {
	for _, bad := range []struct {
		name string
		typ  domain.FieldType
		opts []string
	}{
		{"", domain.FieldText, nil}, {strings.Repeat("n", 61), domain.FieldText, nil}, {"X", "colour", nil},
		{"Tier", domain.FieldSelect, nil}, {"Tier", domain.FieldSelect, []string{"Gold", "gold"}}, {"Tier", domain.FieldSelect, []string{" "}},
		{"Notes", domain.FieldText, []string{"a"}}, {"status", domain.FieldText, nil}, {"Story Points", domain.FieldNumber, nil},
	} {
		if _, err := domain.NewCustomField("f", "KYB", bad.name, bad.typ, bad.opts); !errors.Is(err, domain.ErrInvalidCustomField) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	f, err := domain.NewCustomField("f-1", "KYB", " Tier ", domain.FieldSelect, []string{" Gold ", "Silver"})
	if err != nil || f.Name() != "Tier" || f.Type() != domain.FieldSelect || strings.Join(f.Options(), ",") != "Gold,Silver" || f.ID() != "f-1" || f.Project() != "KYB" || f.Version() != 0 {
		t.Fatalf("new = %+v %v", f, err)
	}
	if err := f.Edit("Level", []string{"Gold", "Bronze"}); err != nil || f.Name() != "Level" || f.Options()[1] != "Bronze" {
		t.Fatalf("edit: %v", err)
	}
	if err := f.Edit("", nil); !errors.Is(err, domain.ErrInvalidCustomField) || f.Name() != "Level" {
		t.Fatalf("bad edit: %v", err)
	}
	if f.MarkPersisted(); f.Version() != 1 {
		t.Fatal("version")
	}
	back := domain.RehydrateCustomField(f.Snapshot())
	if back.Name() != "Level" || back.Version() != 1 || back.Type() != domain.FieldSelect {
		t.Fatalf("rehydrate = %+v", back.Snapshot())
	}
}

func TestCustomFieldValues(t *testing.T) {
	text, _ := domain.NewCustomField("f-t", "KYB", "Customer", domain.FieldText, nil)
	num, _ := domain.NewCustomField("f-n", "KYB", "Budget", domain.FieldNumber, nil)
	date, _ := domain.NewCustomField("f-d", "KYB", "Due", domain.FieldDate, nil)
	sel, _ := domain.NewCustomField("f-s", "KYB", "Tier", domain.FieldSelect, []string{"Gold", "Silver"})
	other, _ := domain.NewCustomField("f-o", "OPS", "Elsewhere", domain.FieldText, nil)
	for _, c := range []struct {
		f        *domain.CustomField
		in, want string
	}{
		{text, "  Acme  ", "Acme"}, {num, "12.50", "12.5"}, {num, "-3", "-3"}, {date, "2026-12-01", "2026-12-01"}, {sel, "gold", "Gold"},
	} {
		got, err := c.f.Normalize(c.in)
		if err != nil || got != c.want {
			t.Errorf("%s %q = %q %v", c.f.Name(), c.in, got, err)
		}
	}
	for _, c := range []struct {
		f  *domain.CustomField
		in string
	}{{num, "lots"}, {num, "NaN"}, {date, "1 Dec"}, {date, "2026-13-01"}, {sel, "Bronze"}, {text, strings.Repeat("x", 1001)}} {
		if _, err := c.f.Normalize(c.in); !errors.Is(err, domain.ErrInvalidFieldValue) {
			t.Errorf("%s %q: %v", c.f.Name(), c.in, err)
		}
	}

	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo, Version: 1})
	if err := is.SetField(sel, "silver"); err != nil || is.Field("f-s") != "Silver" {
		t.Fatalf("set: %v %q", err, is.Field("f-s"))
	}
	if err := is.SetField(num, "x"); !errors.Is(err, domain.ErrInvalidFieldValue) {
		t.Fatalf("bad value: %v", err)
	}
	if err := is.SetField(other, "y"); !errors.Is(err, domain.ErrInvalidFieldValue) {
		t.Fatalf("another project's field: %v", err)
	}
	_ = is.SetField(text, "Acme")
	_ = is.SetField(text, "") // clears
	ch, _ := is.Changelog("u-1")
	if len(ch.Items) != 1 || ch.Items[0].Field != "custom:Tier" || ch.Items[0].To != "Silver" {
		t.Fatalf("changelog = %+v", ch.Items)
	}
	if got := is.Fields(); len(got) != 1 || got["f-s"] != "Silver" {
		t.Fatalf("fields = %v", got)
	}
	if back := domain.Rehydrate(is.Snapshot()); back.Field("f-s") != "Silver" {
		t.Fatal("snapshot lost custom fields")
	}
}
