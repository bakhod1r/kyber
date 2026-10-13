package domain_test

import (
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestFixVersions(t *testing.T) {
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo,
		FixVersions: []domain.VersionID{"v-2"}, Version: 1})
	if !is.HasFixVersion("v-2") || is.HasFixVersion("v-1") {
		t.Fatal("rehydrated fix versions")
	}
	is.PlanForVersions([]domain.VersionID{"v-3", "v-1", "v-3"})
	if got := is.FixVersions(); !slices.Equal(got, []domain.VersionID{"v-1", "v-3"}) {
		t.Fatalf("set = %v", got)
	}
	is.MoveToVersion("v-1", "v-2")
	is.MoveToVersion("v-3", "")
	if got := is.Snapshot().FixVersions; !slices.Equal(got, []domain.VersionID{"v-2"}) {
		t.Fatalf("replaced = %v", got)
	}
	is.PlanForVersions([]domain.VersionID{"v-1", "v-2"})
	ch, ok := is.Changelog("u-1")
	if !ok || len(ch.Items) != 1 || ch.Items[0].Field != "fix_versions" || ch.Items[0].From != "v-2" || ch.Items[0].To != "v-1,v-2" {
		t.Fatalf("changelog = %+v", ch.Items)
	}
}
