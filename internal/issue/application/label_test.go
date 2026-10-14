package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/application"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestLabels(t *testing.T) {
	ctx := context.Background()
	s := seed(t, 3)
	label := func(key string, labels ...string) error {
		is, _ := s.Get(ctx, dev, key)
		_, err := s.Edit(ctx, dev, key, application.EditIssue{Version: is.Version(), Labels: &labels})
		return err
	}
	if err := label("KYB-1", "ui", "backend"); err != nil {
		t.Fatal(err)
	}
	if err := label("KYB-2", "ui"); err != nil {
		t.Fatal(err)
	}
	if err := label("KYB-3", "two words"); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Fatalf("invalid label: %v", err)
	}
	if got := order(t, s, application.ListQuery{Label: "ui"}); got != "KYB-1,KYB-2" {
		t.Fatalf("labelled ui = %s", got)
	}
	if got := order(t, s, application.ListQuery{LabelKeys: []string{"backend"}, Types: []string{"task"}, Priorities: []string{"medium"}}); got != "KYB-1" {
		t.Fatalf("pushdown filter = %s", got)
	}
	got, err := s.ProjectLabels(ctx, dev, "KYB")
	if err != nil || !slices.Equal(got, []string{"backend", "ui"}) {
		t.Fatalf("ProjectLabels = %v %v", got, err)
	}
}
