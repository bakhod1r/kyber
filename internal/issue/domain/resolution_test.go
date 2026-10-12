package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestResolution(t *testing.T) {
	wf := domain.DefaultWorkflow()
	is, _ := domain.NewIssue("i-1", mustKey(t, "KYB-1"), "Login", domain.TypeTask, "u-1", wf)
	if is.Resolution() != "" {
		t.Fatal("a new issue is unresolved")
	}
	// A resolution needs a Done status.
	if err := is.TransitionResolved(domain.StatusInProgress, wf, domain.ResolutionWontDo); !errors.Is(err, domain.ErrInvalidResolution) {
		t.Fatalf("resolution outside Done: %v", err)
	}
	if err := is.Transition(domain.StatusDone, wf); err != nil || is.Resolution() != domain.ResolutionDone {
		t.Fatalf("entering Done resolves as done: %q %v", is.Resolution(), err)
	}
	if err := is.TransitionResolved(domain.StatusDone, wf, domain.ResolutionDuplicate); err == nil {
		t.Fatal("done → done is not a transition")
	}
	if err := is.Transition(domain.StatusTodo, wf); err != nil || is.Resolution() != "" {
		t.Fatalf("reopening clears the resolution: %q %v", is.Resolution(), err)
	}
	if err := is.TransitionResolved(domain.StatusDone, wf, domain.ResolutionCannotReproduce); err != nil || is.Resolution() != domain.ResolutionCannotReproduce {
		t.Fatalf("resolved as %q %v", is.Resolution(), err)
	}

	for _, ok := range []string{"done", "wont_do", "duplicate", "cannot_reproduce"} {
		if r, err := domain.ParseResolution(ok); err != nil || string(r) != ok {
			t.Errorf("%s: %v", ok, err)
		}
	}
	if _, err := domain.ParseResolution("fixed"); !errors.Is(err, domain.ErrInvalidResolution) {
		t.Fatalf("unknown resolution: %v", err)
	}
	if err := is.SetResolution(domain.ResolutionDuplicate); err != nil || is.Resolution() != domain.ResolutionDuplicate {
		t.Fatalf("change resolution: %v", err)
	}
	if err := is.SetResolution("fixed"); !errors.Is(err, domain.ErrInvalidResolution) {
		t.Fatalf("unknown: %v", err)
	}
	_ = is.Transition(domain.StatusTodo, wf)
	if err := is.SetResolution(domain.ResolutionDone); !errors.Is(err, domain.ErrInvalidResolution) {
		t.Fatalf("unresolved issue got a resolution: %v", err)
	}
	if len(domain.Resolutions()) != 4 {
		t.Fatal("four resolutions")
	}
}

func TestResolutionInChangelogAndSnapshot(t *testing.T) {
	wf := domain.DefaultWorkflow()
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "x", Type: domain.TypeTask, Status: domain.StatusTodo, Version: 1})
	_ = is.TransitionResolved(domain.StatusDone, wf, domain.ResolutionWontDo)
	if is.Snapshot().Resolution != domain.ResolutionWontDo {
		t.Fatal("snapshot lacks the resolution")
	}
	ch, ok := is.Changelog("u-1")
	if !ok || len(ch.Items) != 2 || ch.Items[1].Field != "resolution" || ch.Items[1].To != "wont_do" {
		t.Fatalf("changelog = %+v", ch.Items)
	}
	back := domain.Rehydrate(is.Snapshot())
	if back.Resolution() != domain.ResolutionWontDo {
		t.Fatal("rehydrate drops the resolution")
	}
}

func mustKey(t *testing.T, s string) domain.IssueKey {
	t.Helper()
	k, err := domain.ParseIssueKey(s)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
