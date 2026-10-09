package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/adapter/memory"
	"github.com/bakhod1r/kyber/internal/agile/app"
	"github.com/bakhod1r/kyber/internal/agile/domain"
)

const (
	dev    = "u-dev"
	viewer = "u-viewer"
	alien  = "u-alien"
)

type fakeAccess map[string]map[string]string

func (f fakeAccess) Authorize(_ context.Context, actor, project string, write bool) error {
	role, ok := f[project][actor]
	if !ok {
		return app.ErrProjectNotFound
	}
	if write && role != "w" {
		return app.ErrForbidden
	}
	return nil
}

type fakeIssues struct {
	calls     []string
	completed int
	returned  int
	err       error
}

func (f *fakeIssues) ReturnUnfinished(_ context.Context, project, sprint string) (int, int, error) {
	f.calls = append(f.calls, project+"/"+sprint)
	return f.completed, f.returned, f.err
}

func setup() (*app.Service, *memory.Repository, *fakeIssues) {
	repo := memory.NewRepository()
	issues := &fakeIssues{completed: 3, returned: 2}
	n := 0
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	svc := app.NewService(app.Deps{
		Sprints: repo, Issues: issues,
		Access: fakeAccess{"KYB": {dev: "w", viewer: "r"}, "OPS": {dev: "w"}},
		NewID:  func() string { n++; return fmt.Sprintf("s-%d", n) },
		Now:    func() time.Time { now = now.Add(time.Hour); return now },
	})
	return svc, repo, issues
}

func TestCreateAndList(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	sp, err := s.Create(ctx, dev, "KYB", "Sprint 1", "Ship")
	if err != nil || sp.State() != domain.StatePlanned {
		t.Fatalf("Create = %+v, %v", sp, err)
	}
	_, _ = s.Create(ctx, dev, "KYB", "Sprint 2", "")
	list, err := s.List(ctx, viewer, "KYB")
	if err != nil || len(list) != 2 || list[0].Name() != "Sprint 1" {
		t.Fatalf("List = %v, %v", list, err)
	}
	if _, err := s.Create(ctx, viewer, "KYB", "x", ""); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer create err = %v", err)
	}
	if _, err := s.List(ctx, alien, "KYB"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("outsider list err = %v", err)
	}
	if _, err := s.Create(ctx, dev, "KYB", " ", ""); !errors.Is(err, domain.ErrInvalidSprint) {
		t.Fatalf("blank name err = %v", err)
	}
}

func TestStartOneActivePerProject(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	a, _ := s.Create(ctx, dev, "KYB", "A", "")
	b, _ := s.Create(ctx, dev, "KYB", "B", "")
	o, _ := s.Create(ctx, dev, "OPS", "O", "")

	if _, err := s.Start(ctx, dev, string(a.ID())); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, dev, string(b.ID())); !errors.Is(err, domain.ErrAnotherSprintLive) {
		t.Fatalf("second start err = %v", err)
	}
	if _, err := s.Start(ctx, dev, string(o.ID())); err != nil {
		t.Fatalf("other project: %v", err)
	}
	if _, err := s.Start(ctx, dev, string(a.ID())); !errors.Is(err, domain.ErrSprintState) {
		t.Fatalf("restart err = %v", err)
	}
	if _, err := s.Start(ctx, viewer, string(b.ID())); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer start err = %v", err)
	}
	// Outsiders cannot tell a sprint exists.
	if _, err := s.Start(ctx, alien, string(b.ID())); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
	if _, err := s.Start(ctx, dev, "s-404"); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestComplete(t *testing.T) {
	ctx := context.Background()
	s, repo, issues := setup()
	a, _ := s.Create(ctx, dev, "KYB", "A", "")
	if _, err := s.Complete(ctx, dev, string(a.ID())); !errors.Is(err, domain.ErrSprintState) {
		t.Fatalf("complete planned err = %v", err)
	}
	if len(issues.calls) != 0 {
		t.Fatal("issues must not move when completion is rejected")
	}
	_, _ = s.Start(ctx, dev, string(a.ID()))
	res, err := s.Complete(ctx, dev, string(a.ID()))
	if err != nil || res.Completed != 3 || res.Returned != 2 || res.Sprint.State() != domain.StateClosed {
		t.Fatalf("Complete = %+v, %v", res, err)
	}
	if len(issues.calls) != 1 || issues.calls[0] != "KYB/"+string(a.ID()) {
		t.Fatalf("ReturnUnfinished calls = %v", issues.calls)
	}
	if stored, _ := repo.ByID(ctx, a.ID()); stored.State() != domain.StateClosed {
		t.Fatal("sprint not closed")
	}
	if got := repo.OutboxNames(); len(got) != 3 || got[2] != "sprint.completed" {
		t.Fatalf("outbox = %v", got)
	}

	// If returning issues fails the sprint stays active, so completion can be retried.
	b, _ := s.Create(ctx, dev, "KYB", "B", "")
	_, _ = s.Start(ctx, dev, string(b.ID()))
	issues.err = errors.New("db down")
	if _, err := s.Complete(ctx, dev, string(b.ID())); err == nil {
		t.Fatal("expected error")
	}
	if stored, _ := repo.ByID(ctx, b.ID()); stored.State() != domain.StateActive {
		t.Fatal("sprint must stay active when issues could not be returned")
	}
}

func TestCanHold(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	a, _ := s.Create(ctx, dev, "KYB", "A", "")
	if err := s.CanHold(ctx, "KYB", string(a.ID())); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ project, sprint string }{{"OPS", string(a.ID())}, {"KYB", "s-404"}} {
		if err := s.CanHold(ctx, c.project, c.sprint); !errors.Is(err, app.ErrInvalidSprint) {
			t.Fatalf("CanHold(%v) err = %v", c, err)
		}
	}
	_, _ = s.Start(ctx, dev, string(a.ID()))
	_, _ = s.Complete(ctx, dev, string(a.ID()))
	if err := s.CanHold(ctx, "KYB", string(a.ID())); !errors.Is(err, app.ErrInvalidSprint) {
		t.Fatalf("closed sprint err = %v", err)
	}
}
