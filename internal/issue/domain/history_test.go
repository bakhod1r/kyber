package domain_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestChangelogRecordsEveryFieldWithOldAndNewValues(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	p3 := domain.Points(30)
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: key, Title: "Old", Type: domain.TypeTask, Status: "todo",
		Description: "d", Priority: domain.PriorityMedium, Assignee: "u-a", Estimate: &p3, Rank: "a0", Sprint: "s-1", Version: 2})
	if _, ok := is.Changelog("u-lead"); ok {
		t.Fatal("no change → no changelog")
	}
	title, desc, prio := "New", "d2", domain.PriorityHigh
	if err := is.Edit(domain.Changes{Title: &title, Description: &desc, Priority: &prio}); err != nil {
		t.Fatal(err)
	}
	is.Assign("u-b", "u-lead")
	_ = is.Transition("in_progress", domain.DefaultWorkflow())
	p5 := domain.Points(50)
	is.SetEstimate(&p5)
	is.MoveToSprint("")
	is.Rerank("a5")
	ch, ok := is.Changelog("u-lead")
	if !ok || ch.By != "u-lead" || ch.Key != key || ch.ID != "i-1" {
		t.Fatalf("changelog = %+v", ch)
	}
	want := []domain.FieldChange{
		{Field: "title", From: "Old", To: "New"},
		{Field: "description", From: "d", To: "d2"},
		{Field: "status", From: "todo", To: "in_progress"},
		{Field: "priority", From: "medium", To: "high"},
		{Field: "assignee", From: "u-a", To: "u-b"},
		{Field: "estimate", From: "3", To: "5"},
		{Field: "sprint", From: "s-1", To: ""},
		{Field: "rank", From: "a0", To: "a5"},
	}
	if len(ch.Items) != len(want) {
		t.Fatalf("items = %+v", ch.Items)
	}
	for i := range want {
		if ch.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, ch.Items[i], want[i])
		}
	}
	if ch.EventName() != "issue.changed" {
		t.Fatal(ch.EventName())
	}
	// After the save the baseline moves on: the next changelog only has new changes.
	is.MarkPersisted()
	if _, ok := is.Changelog("u-lead"); ok {
		t.Fatal("persisted changes must not be logged twice")
	}
	is.SetEstimate(nil)
	if ch, _ := is.Changelog("u-x"); len(ch.Items) != 1 || ch.Items[0] != (domain.FieldChange{Field: "estimate", From: "5", To: ""}) {
		t.Fatalf("estimate cleared = %+v", ch.Items)
	}
}

func TestNewIssuesHaveNoChangelog(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	is, _ := domain.NewIssue("i-1", key, "t", domain.TypeTask, "u-1", domain.DefaultWorkflow())
	if _, ok := is.Changelog("u-1"); ok {
		t.Fatal("creation is recorded by issue.created, not a changelog")
	}
}

func TestSecurityLevel(t *testing.T) {
	key, _ := domain.NewIssueKey("KYB", 1)
	is := domain.Rehydrate(domain.Snapshot{ID: "i", Key: key, Title: "t", Type: domain.TypeTask, Status: "todo", Version: 1})
	if is.SecurityLevel() != "" {
		t.Fatal("visible by default")
	}
	is.Classify("confidential")
	ch, ok := is.Changelog("u")
	if !ok || ch.Items[0] != (domain.FieldChange{Field: "security", From: "", To: "confidential"}) || is.Snapshot().Security != "confidential" {
		t.Fatalf("changelog = %+v", ch)
	}
	again := domain.Rehydrate(is.Snapshot())
	if again.SecurityLevel() != "confidential" {
		t.Fatal("rehydrate lost the level")
	}
}
