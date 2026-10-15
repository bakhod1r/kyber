package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

func TestParseTemplate(t *testing.T) {
	for in, want := range map[string]domain.Template{"": domain.TemplateKanban, "kanban": domain.TemplateKanban, "scrum": domain.TemplateScrum, "bug-tracking": domain.TemplateBugTracking} {
		if got, err := domain.ParseTemplate(in); err != nil || got != want {
			t.Errorf("ParseTemplate(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := domain.ParseTemplate("waterfall"); !errors.Is(err, domain.ErrInvalidTemplate) {
		t.Fatalf("unknown template: %v", err)
	}
}
