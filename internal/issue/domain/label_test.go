package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestLabels(t *testing.T) {
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo,
		Labels: []domain.Label{"ui"}, Version: 1})
	if !is.HasLabel("ui") || is.HasLabel("api") {
		t.Fatal("rehydrated labels")
	}
	if err := is.Relabel([]string{" backend ", "ui", "backend"}); err != nil {
		t.Fatal(err)
	}
	if got := is.Labels(); !slices.Equal(got, []domain.Label{"backend", "ui"}) {
		t.Fatalf("labels = %v", got)
	}
	ch, ok := is.Changelog("u-1")
	if !ok || len(ch.Items) != 1 || ch.Items[0].Field != "labels" || ch.Items[0].From != "ui" || ch.Items[0].To != "backend,ui" {
		t.Fatalf("changelog = %+v", ch.Items)
	}
	for _, bad := range []string{"", "two words", "a,b", string(make([]byte, 101))} {
		if err := is.Relabel([]string{bad}); !errors.Is(err, domain.ErrInvalidLabel) {
			t.Errorf("Relabel(%q) = %v", bad, err)
		}
	}
	many := make([]string, 51)
	for n := range many {
		many[n] = string(rune('a'+n%26)) + string(rune('a'+n/26))
	}
	if err := is.Relabel(many); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("too many = %v", err)
	}
	if got := is.Labels(); !slices.Equal(got, []domain.Label{"backend", "ui"}) {
		t.Fatalf("rejected relabel changed labels: %v", got)
	}
}
