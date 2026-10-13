package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bakhod1r/kyber/internal/goal/application"
	"github.com/bakhod1r/kyber/internal/goal/domain"
	"github.com/bakhod1r/kyber/internal/goal/infrastructure/memory"
)

const (
	lead   = "u-lead"
	viewer = "u-viewer"
	out    = "u-outsider"
)

// projects: lead writes KYB, viewer only browses it, the outsider sees nothing.
type projects struct{}

func (projects) Authorize(_ context.Context, actor, project string, change bool) error {
	switch {
	case project != "KYB" || actor == out:
		return application.ErrProjectNotFound
	case change && actor == viewer:
		return application.ErrForbidden
	}
	return nil
}

// issues: "type = bug" finds 3 issues, 1 done; "broken" is invalid; "boom" fails.
type issues struct{}

var errBoom = errors.New("boom")

func (issues) Measure(_ context.Context, _, _, jql string) (domain.Progress, error) {
	switch jql {
	case "broken":
		return domain.Progress{}, fmt.Errorf("%w: JQL at 0", application.ErrInvalidQuery)
	case "boom":
		return domain.Progress{}, errBoom
	}
	return domain.Progress{Done: 1, Total: 3}, nil
}

type failing struct{ *memory.Repository }

func (failing) ForProject(context.Context, string) ([]*domain.Goal, error) { return nil, errBoom }

func setup() (*application.Service, *memory.Repository) {
	repo := memory.NewRepository()
	n := 0
	return application.NewService(application.Deps{Projects: projects{}, Issues: issues{}, Goals: repo,
		NewID: func() string { n++; return fmt.Sprintf("g-%d", n) }}), repo
}

func TestGoals(t *testing.T) {
	ctx := context.Background()
	s, repo := setup()

	v, err := s.Create(ctx, lead, "KYB", "Fewer bugs", "2026-12-31", "type = bug")
	if err != nil || v.Progress.Total != 3 || v.Goal.Owner != lead || v.Goal.Version != 1 {
		t.Fatalf("create = %+v %v", v, err)
	}
	if _, err := s.Create(ctx, lead, "KYB", "Plain", "", ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor, project, name, jql string
		want                      error
	}{
		{viewer, "KYB", "x", "", application.ErrForbidden},
		{out, "KYB", "x", "", application.ErrProjectNotFound},
		{lead, "KYB", "", "", domain.ErrInvalidGoal},
		{lead, "KYB", "x", "broken", application.ErrInvalidQuery},
	} {
		if _, err := s.Create(ctx, tc.actor, tc.project, tc.name, "", tc.jql); !errors.Is(err, tc.want) {
			t.Errorf("create %+v: %v", tc, err)
		}
	}

	list, err := s.Goals(ctx, viewer, "KYB")
	if err != nil || len(list) != 2 || list[0].Goal.Name != "Fewer bugs" || list[0].Progress.Done != 1 || list[1].Progress.Total != 0 {
		t.Fatalf("goals = %+v %v", list, err)
	}
	if _, err := s.Goals(ctx, out, "KYB"); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider lists: %v", err)
	}

	id := string(v.Goal.ID)
	up, err := s.Update(ctx, lead, id, application.Change{Version: 1, Name: "Fewer bugs", Target: "", JQL: "type = bug", Status: domain.StatusAtRisk})
	if err != nil || up.Goal.Status != domain.StatusAtRisk || up.Goal.Version != 2 {
		t.Fatalf("update = %+v %v", up, err)
	}
	for _, tc := range []struct {
		actor string
		c     application.Change
		want  error
	}{
		{lead, application.Change{Version: 1, Name: "x", Status: domain.StatusDone}, domain.ErrGoalConflict},
		{lead, application.Change{Version: 2, Name: "", Status: domain.StatusDone}, domain.ErrInvalidGoal},
		{lead, application.Change{Version: 2, Name: "x", Status: "great"}, domain.ErrInvalidGoal},
		{lead, application.Change{Version: 2, Name: "x", JQL: "broken", Status: domain.StatusDone}, application.ErrInvalidQuery},
		{viewer, application.Change{Version: 2, Name: "x", Status: domain.StatusDone}, application.ErrForbidden},
		{out, application.Change{Version: 2, Name: "x", Status: domain.StatusDone}, domain.ErrGoalNotFound},
	} {
		if _, err := s.Update(ctx, tc.actor, id, tc.c); !errors.Is(err, tc.want) {
			t.Errorf("update %s %+v: %v", tc.actor, tc.c, err)
		}
	}
	if _, err := s.Update(ctx, lead, "nope", application.Change{}); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("missing goal: %v", err)
	}

	// A query that broke later shows no progress; a failing measurement is an error.
	g, _ := repo.ByID(ctx, v.Goal.ID)
	g.JQL = "broken"
	_ = repo.Save(ctx, g)
	if list, err := s.Goals(ctx, lead, "KYB"); err != nil || list[0].Progress.Total != 0 {
		t.Fatalf("broken query = %+v %v", list, err)
	}
	g.JQL = "boom"
	_ = repo.Save(ctx, g)
	if _, err := s.Goals(ctx, lead, "KYB"); !errors.Is(err, errBoom) {
		t.Fatalf("measure failure: %v", err)
	}

	if err := s.Delete(ctx, viewer, id); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer deletes: %v", err)
	}
	if err := s.Delete(ctx, lead, id); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, lead, id); !errors.Is(err, domain.ErrGoalNotFound) {
		t.Fatalf("delete twice: %v", err)
	}

	bad := application.NewService(application.Deps{Projects: projects{}, Issues: issues{}, Goals: failing{memory.NewRepository()}})
	if _, err := bad.Goals(ctx, lead, "KYB"); !errors.Is(err, errBoom) {
		t.Fatalf("repository failure: %v", err)
	}
}
