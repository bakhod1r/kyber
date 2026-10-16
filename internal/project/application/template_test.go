package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

type appliedTemplates []string

func (a *appliedTemplates) Apply(_ context.Context, actor domain.UserID, key string, t domain.Template) error {
	*a = append(*a, string(actor)+"/"+key+"/"+string(t))
	return nil
}

// B10: a project starts from a template; the contexts that own the workflow and the board set it up.
func TestCreateFromTemplate(t *testing.T) {
	ctx := context.Background()
	applied := &appliedTemplates{}
	s := newService(t).WithTemplates(applied)
	p, err := s.CreateFromTemplate(ctx, alice, "BUG", "Bugs", "bug-tracking")
	if err != nil || p.Key() != "BUG" {
		t.Fatalf("Create = %v, %v", p, err)
	}
	if len(*applied) != 1 || (*applied)[0] != string(alice)+"/BUG/bug-tracking" {
		t.Fatalf("applied = %v", *applied)
	}
	if _, err := s.CreateFromTemplate(ctx, alice, "OPS", "Ops", "waterfall"); !errors.Is(err, domain.ErrInvalidTemplate) {
		t.Fatalf("unknown template: %v", err)
	}
	if _, err := s.Get(ctx, alice, "OPS"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatal("a refused template must not leave a project behind")
	}
	// Kanban is what every project starts as: nothing to apply.
	if _, err := s.CreateFromTemplate(ctx, alice, "KAN", "Kan", ""); err != nil || len(*applied) != 1 {
		t.Fatalf("kanban = %v, applied %v", err, *applied)
	}
}
