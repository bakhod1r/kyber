package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/search/domain"
)

func TestFilter(t *testing.T) {
	f, err := domain.NewFilter("f-1", "u-1", "  My bugs ", " type = bug ", false)
	if err != nil || f.Name != "My bugs" || f.JQL != "type = bug" || f.Owner != "u-1" || f.Shared {
		t.Fatalf("new = %+v %v", f, err)
	}
	for _, name := range []string{"", "  ", strings.Repeat("n", 61)} {
		if _, err := domain.NewFilter("f", "u", name, "", false); !errors.Is(err, domain.ErrInvalidFilter) {
			t.Errorf("name %q: %v", name, err)
		}
	}
	if err := f.Edit("x", "project ==", true); !errors.Is(err, domain.ErrInvalidJQL) || f.Name != "My bugs" {
		t.Fatalf("bad jql: %v (name %q)", err, f.Name)
	}
	if err := f.Edit("Team bugs", "type = bug order by priority desc", true); err != nil || !f.Shared {
		t.Fatalf("edit: %v", err)
	}
}
