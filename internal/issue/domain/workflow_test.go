package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func reviewFlow(t *testing.T) *domain.Workflow {
	t.Helper()
	wf, err := domain.DefineWorkflow(domain.WorkflowDef{
		Initial: "backlog",
		Statuses: []domain.Status{
			{ID: "backlog", Name: "Backlog", Category: domain.CategoryToDo},
			{ID: "dev", Name: "In Development", Category: domain.CategoryInProgress},
			{ID: "review", Name: "Code Review", Category: domain.CategoryInProgress},
			{ID: "closed", Name: "Closed", Category: domain.CategoryDone},
		},
		Transitions: []domain.Transition{
			{From: "backlog", To: "dev"}, {From: "dev", To: "review"},
			{From: "review", To: "dev"}, {From: "review", To: "closed"}, {From: "closed", To: "backlog"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return wf
}

func TestDefineWorkflow(t *testing.T) {
	wf := reviewFlow(t)
	cases := []struct {
		from, to domain.StatusID
		ok       bool
	}{
		{"backlog", "dev", true}, {"dev", "review", true}, {"review", "closed", true},
		{"backlog", "closed", false}, {"dev", "closed", false}, {"closed", "dev", false},
	}
	for _, c := range cases {
		if got := wf.CanTransition(c.from, c.to); got != c.ok {
			t.Errorf("%s -> %s = %v, want %v", c.from, c.to, got, c.ok)
		}
	}
	if wf.Initial() != "backlog" || wf.CategoryOf("review") != domain.CategoryInProgress ||
		!wf.IsDone("closed") || wf.IsDone("review") || wf.CategoryOf("ghost") != "" {
		t.Fatalf("categories wrong: %+v", wf.Statuses())
	}
	if got := wf.Statuses(); len(got) != 4 || got[0].ID != "backlog" || got[3].Name != "Closed" {
		t.Fatalf("statuses keep definition order, got %+v", got)
	}
	if got := wf.Transitions(); len(got) != 5 || got[0] != (domain.Transition{From: "backlog", To: "dev"}) {
		t.Fatalf("transitions = %+v", got)
	}
}

func TestDefineWorkflowValidation(t *testing.T) {
	ok := []domain.Status{{ID: "a", Name: "A", Category: domain.CategoryToDo}, {ID: "b", Name: "B", Category: domain.CategoryDone}}
	cases := []struct {
		name string
		def  domain.WorkflowDef
		want error
	}{
		{"no statuses", domain.WorkflowDef{Initial: "a"}, domain.ErrInvalidWorkflow},
		{"unknown initial", domain.WorkflowDef{Initial: "z", Statuses: ok}, domain.ErrUnknownStatus},
		{"empty id", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{Name: "X", Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"bad id", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "Has Space", Name: "X", Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"empty name", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "c", Name: " ", Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"long name", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "c", Name: string(make([]byte, 61)), Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"bad category", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "c", Name: "C", Category: "later"})}, domain.ErrInvalidWorkflow},
		{"duplicate id", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "a", Name: "Again", Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"duplicate name", domain.WorkflowDef{Initial: "a", Statuses: append(ok, domain.Status{ID: "c", Name: "a", Category: domain.CategoryToDo})}, domain.ErrInvalidWorkflow},
		{"no done status", domain.WorkflowDef{Initial: "a", Statuses: ok[:1]}, domain.ErrInvalidWorkflow},
		{"transition to unknown", domain.WorkflowDef{Initial: "a", Statuses: ok, Transitions: []domain.Transition{{From: "a", To: "z"}}}, domain.ErrUnknownStatus},
		{"transition from unknown", domain.WorkflowDef{Initial: "a", Statuses: ok, Transitions: []domain.Transition{{From: "z", To: "a"}}}, domain.ErrUnknownStatus},
		{"self transition", domain.WorkflowDef{Initial: "a", Statuses: ok, Transitions: []domain.Transition{{From: "a", To: "a"}}}, domain.ErrInvalidWorkflow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := domain.DefineWorkflow(c.def); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// No transitions listed = Jira's "all statuses can transition to this one" (any-to-any).
func TestDefineWorkflowWithoutTransitionsIsAnyToAny(t *testing.T) {
	wf, err := domain.DefineWorkflow(domain.WorkflowDef{Initial: "a", Statuses: []domain.Status{
		{ID: "a", Name: "A", Category: domain.CategoryToDo}, {ID: "b", Name: "B", Category: domain.CategoryDone}}})
	if err != nil {
		t.Fatal(err)
	}
	if !wf.CanTransition("a", "b") || !wf.CanTransition("b", "a") || wf.CanTransition("a", "a") {
		t.Fatal("expected any-to-any between distinct statuses")
	}
	if len(wf.Transitions()) != 0 {
		t.Fatalf("definition keeps no explicit transitions, got %v", wf.Transitions())
	}
}

func TestDefaultWorkflowCategories(t *testing.T) {
	wf := domain.DefaultWorkflow()
	if wf.CategoryOf(domain.StatusTodo) != domain.CategoryToDo || wf.CategoryOf(domain.StatusInProgress) != domain.CategoryInProgress ||
		!wf.IsDone(domain.StatusDone) || len(wf.Statuses()) != 3 || wf.Statuses()[1].Name != "In Progress" {
		t.Fatalf("default workflow = %+v", wf.Statuses())
	}
	if !domain.CategoryDone.Valid() || domain.Category("x").Valid() {
		t.Fatal("category validity")
	}
}
