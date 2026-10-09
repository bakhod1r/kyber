package domain_test

import (
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestMoveToSprint(t *testing.T) {
	is := newIssue(t)
	is.PullEvents()
	is.MoveToSprint("s-1")
	is.MoveToSprint("s-1") // no-op
	is.MoveToSprint("")    // back to backlog
	ev := is.PullEvents()
	if len(ev) != 2 {
		t.Fatalf("events = %+v", ev)
	}
	a, b := ev[0].(domain.IssueSprintChanged), ev[1].(domain.IssueSprintChanged)
	if a.From != "" || a.To != "s-1" || b.From != "s-1" || b.To != "" || is.Sprint() != "" {
		t.Fatalf("events = %+v %+v", a, b)
	}
	if a.EventName() != "issue.sprint_changed" {
		t.Fatalf("name = %q", a.EventName())
	}
}

func TestRerank(t *testing.T) {
	is := newIssue(t)
	is.Rerank("a5")
	if is.Rank() != "a5" {
		t.Fatalf("rank = %q", is.Rank())
	}
	key, _ := domain.NewIssueKey("KYB", 2)
	r := domain.Rehydrate(domain.Snapshot{ID: "i-2", Key: key, Title: "t", Type: domain.TypeTask, Status: domain.StatusTodo, Rank: "b12", Sprint: "s-9", Version: 1})
	if r.Rank() != "b12" || r.Sprint() != "s-9" {
		t.Fatalf("rehydrated rank/sprint = %q %q", r.Rank(), r.Sprint())
	}
}
