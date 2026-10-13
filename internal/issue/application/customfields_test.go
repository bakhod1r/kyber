package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestCustomFields(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	spec := application.FieldSpec{Name: "Tier", Type: "select", Options: []string{"Gold", "Silver"}}
	for who, want := range map[string]error{dev: application.ErrForbidden, "u-alien": application.ErrProjectNotFound} {
		if _, err := s.CreateCustomField(ctx, who, "KYB", spec); !errors.Is(err, want) {
			t.Errorf("create as %s: %v", who, err)
		}
	}
	if _, err := s.CreateCustomField(ctx, lead, "KYB", application.FieldSpec{Name: "x", Type: "colour"}); !errors.Is(err, domain.ErrInvalidCustomField) {
		t.Fatalf("bad type: %v", err)
	}
	tier, err := s.CreateCustomField(ctx, lead, "KYB", spec)
	if err != nil {
		t.Fatal(err)
	}
	budget, _ := s.CreateCustomField(ctx, lead, "KYB", application.FieldSpec{Name: "Budget", Type: "number"})
	list, err := s.CustomFields(ctx, viewer, "KYB")
	if err != nil || len(list) != 2 || list[0].Name() != "Tier" {
		t.Fatalf("list = %v %v", list, err)
	}
	if _, err := s.CustomFields(ctx, "u-alien", "KYB"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider list: %v", err)
	}

	is, _ := s.Create(ctx, dev, application.CreateIssue{Project: "KYB", Title: "Deal", Type: "task"})
	key := is.Key().String()
	edit := func(v map[string]string) (*domain.Issue, error) {
		cur, _ := s.Get(ctx, dev, key)
		return s.Edit(ctx, dev, key, application.EditIssue{Version: cur.Version(), Fields: v})
	}
	if _, err := edit(map[string]string{string(budget.ID()): "lots"}); !errors.Is(err, domain.ErrInvalidFieldValue) {
		t.Fatalf("bad value: %v", err)
	}
	if _, err := edit(map[string]string{"f-404": "x"}); !errors.Is(err, domain.ErrInvalidFieldValue) {
		t.Fatalf("unknown field: %v", err)
	}
	got, err := edit(map[string]string{string(tier.ID()): "gold", string(budget.ID()): "1200"})
	if err != nil || got.Field(tier.ID()) != "Gold" || got.Field(budget.ID()) != "1200" {
		t.Fatalf("set: %v %v", got.Fields(), err)
	}

	if _, err := s.EditCustomField(ctx, dev, string(tier.ID()), 1, spec); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member edits a field: %v", err)
	}
	if _, err := s.EditCustomField(ctx, "u-alien", string(tier.ID()), 1, spec); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("outsider edits a field: %v", err)
	}
	if _, err := s.EditCustomField(ctx, lead, string(tier.ID()), 9, spec); !errors.Is(err, domain.ErrFieldConflict) {
		t.Fatalf("stale: %v", err)
	}
	if _, err := s.EditCustomField(ctx, lead, string(tier.ID()), 1, application.FieldSpec{Name: ""}); !errors.Is(err, domain.ErrInvalidCustomField) {
		t.Fatalf("invalid: %v", err)
	}
	if f, err := s.EditCustomField(ctx, lead, string(tier.ID()), 1, application.FieldSpec{Name: "Level", Options: []string{"Gold"}}); err != nil || f.Name() != "Level" {
		t.Fatalf("edit: %v", err)
	}
	if err := s.DeleteCustomField(ctx, lead, string(budget.ID())); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCustomField(ctx, lead, string(budget.ID())); !errors.Is(err, domain.ErrFieldNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	// Values of deleted fields are no longer shown.
	cur, _ := s.Get(ctx, dev, key)
	if v, err := s.FieldValues(ctx, cur); err != nil || len(v) != 1 || v[string(tier.ID())] != "Gold" {
		t.Fatalf("values = %v %v", v, err)
	}
}
