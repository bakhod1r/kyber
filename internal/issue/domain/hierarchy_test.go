package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func mkIssue(t *testing.T, id string, project string, n int, typ domain.IssueType) *domain.Issue {
	t.Helper()
	key, _ := domain.NewIssueKey(project, n)
	return domain.Rehydrate(domain.Snapshot{ID: domain.IssueID(id), Key: key, Title: "t", Type: typ, Status: "todo", Version: 1})
}

// Jira's hierarchy: epic → story/task/bug → sub-task, within one project.
func TestSetParent(t *testing.T) {
	epic := mkIssue(t, "e", "KYB", 1, domain.TypeEpic)
	story := mkIssue(t, "s", "KYB", 2, domain.TypeStory)
	task := mkIssue(t, "t", "KYB", 3, domain.TypeTask)
	bug := mkIssue(t, "b", "KYB", 4, domain.TypeBug)
	sub := mkIssue(t, "u", "KYB", 5, domain.TypeSubtask)
	otherEpic := mkIssue(t, "o", "OPS", 1, domain.TypeEpic)
	cases := []struct {
		name          string
		child, parent *domain.Issue
		want          error
	}{
		{"story under epic", story, epic, nil},
		{"task under epic", task, epic, nil},
		{"bug under epic", bug, epic, nil},
		{"sub-task under story", sub, story, nil},
		{"sub-task under bug", sub, bug, nil},
		{"sub-task under epic", sub, epic, domain.ErrInvalidParent},
		{"epic under anything", epic, story, domain.ErrInvalidParent},
		{"story under story", story, task, domain.ErrInvalidParent},
		{"under a sub-task", task, sub, domain.ErrInvalidParent},
		{"other project", story, otherEpic, domain.ErrInvalidParent},
		{"itself", story, story, domain.ErrInvalidParent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.child.SetParent(c.parent); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
	if story.Parent() != "e" {
		t.Fatalf("parent = %q", story.Parent())
	}
	ch, ok := story.Changelog("u-1")
	if !ok || ch.Items[0] != (domain.FieldChange{Field: "parent", From: "", To: "KYB-1"}) {
		t.Fatalf("changelog = %+v", ch)
	}
	// Removing the parent is allowed except for sub-tasks, which always need one.
	if err := story.SetParent(nil); err != nil || story.Parent() != "" {
		t.Fatalf("clear = %v", err)
	}
	if err := sub.SetParent(nil); !errors.Is(err, domain.ErrInvalidParent) {
		t.Fatalf("sub-task without parent err = %v", err)
	}
	if snap := sub.Snapshot(); snap.Parent != "b" || snap.ParentKey.String() != "KYB-4" {
		t.Fatalf("snapshot = %+v", snap)
	}
}
