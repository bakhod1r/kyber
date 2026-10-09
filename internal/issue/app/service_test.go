package app_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/adapter/memory"
	"github.com/bakhod1r/kyber/internal/issue/app"
	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// fakeKeys is an in-memory KeyAllocator knowing a fixed set of projects.
type fakeKeys struct {
	mu   sync.Mutex
	seqs map[string]int
}

func (f *fakeKeys) Next(_ context.Context, project string) (domain.IssueKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	seq, ok := f.seqs[project]
	if !ok {
		return domain.IssueKey{}, app.ErrProjectNotFound
	}
	f.seqs[project] = seq + 1
	return domain.NewIssueKey(project, seq+1)
}

// recorder exposes the repository outbox as the events published so far.
type recorder struct {
	repo *memory.Repository
	skip int
}

func (r *recorder) all() []domain.Event { return r.repo.Outbox()[r.skip:] }
func (r *recorder) reset()              { r.skip = len(r.repo.Outbox()) }

// fakeAccess grants per-project roles: "w" = write, "r" = read-only.
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

func (f fakeAccess) IsMember(_ context.Context, project, user string) (bool, error) {
	_, ok := f[project][user]
	return ok, nil
}

// fakeSprints accepts sprint IDs listed per project.
type fakeSprints map[string]map[string]bool

func (f fakeSprints) CanHold(_ context.Context, project, sprint string) error {
	if !f[project][sprint] {
		return app.ErrInvalidSprint
	}
	return nil
}

func (f fakeAccess) DisplayName(_ context.Context, user string) (string, error) {
	return "Name of " + user, nil
}

const (
	dev    = "u-dev"
	viewer = "u-viewer"
	alien  = "u-alien"
)

func setup() (*app.Service, *recorder) {
	repo := memory.NewRepository()
	rec := &recorder{repo: repo}
	n := 0
	ids := func() string { n++; return fmt.Sprintf("i-%d", n) }
	keys := &fakeKeys{seqs: map[string]int{"KYB": 0, "OPS": 0}}
	access := fakeAccess{
		"KYB": {dev: "w", viewer: "r"},
		"OPS": {dev: "w"},
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return app.NewService(app.Deps{
		Issues: repo, Comments: memory.NewCommentRepository(repo), Keys: keys, Access: access, Directory: access,
		Sprints:  fakeSprints{"KYB": {"s-1": true, "s-2": true}},
		Workflow: domain.DefaultWorkflow(), NewID: ids,
		Now: func() time.Time { now = now.Add(time.Minute); return now },
	}), rec
}

func TestCreateIssue(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()

	is1, err := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	if err != nil {
		t.Fatal(err)
	}
	is2, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Logout", Type: "bug"})
	other, _ := s.Create(ctx, dev, app.CreateIssue{Project: "OPS", Title: "Deploy", Type: "story"})

	if is1.Key().String() != "KYB-1" || is2.Key().String() != "KYB-2" || other.Key().String() != "OPS-1" {
		t.Fatalf("keys = %s %s %s", is1.Key(), is2.Key(), other.Key())
	}
	if is1.Reporter() != dev {
		t.Fatalf("reporter = %q, want the creator", is1.Reporter())
	}
	if is1.Status() != domain.StatusTodo {
		t.Fatalf("status = %s", is1.Status())
	}
	if len(rec.all()) != 3 || rec.all()[0].EventName() != "issue.created" {
		t.Fatalf("events = %+v", rec.all())
	}
}

func TestCreateIssueValidation(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	cases := []struct {
		cmd  app.CreateIssue
		want error
	}{
		{app.CreateIssue{Project: "KYB", Title: " ", Type: "task"}, domain.ErrEmptyTitle},
		{app.CreateIssue{Project: "KYB", Title: "x", Type: "feature"}, domain.ErrInvalidIssueType},
		{app.CreateIssue{Project: "NOPE", Title: "x", Type: "task"}, app.ErrProjectNotFound},
	}
	for _, c := range cases {
		if _, err := s.Create(ctx, dev, c.cmd); !errors.Is(err, c.want) {
			t.Errorf("Create(%+v) err = %v, want %v", c.cmd, err, c.want)
		}
	}
	if len(rec.all()) != 0 {
		t.Fatal("failed commands must not publish events")
	}
	// Validation failures must not burn issue numbers.
	is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "ok", Type: "task"})
	if is.Key().Number() != 1 {
		t.Fatalf("number = %d, want 1", is.Key().Number())
	}
}

func TestGetIssue(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})

	is, err := s.Get(ctx, dev, "KYB-1")
	if err != nil || is.Title() != "Login" {
		t.Fatalf("Get = %+v, %v", is, err)
	}
	if _, err := s.Get(ctx, dev, "KYB-9"); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.Get(ctx, dev, "garbage"); !errors.Is(err, app.ErrInvalidKey) {
		t.Fatalf("err = %v", err)
	}
}

func TestTransitionIssue(t *testing.T) {
	ctx := context.Background()
	s, rec := setup()
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	rec.reset()

	if _, err := s.Transition(ctx, dev, "KYB-1", "todo"); !errors.Is(err, domain.ErrTransitionNotAllowed) { // to itself
		t.Fatalf("err = %v", err)
	}
	if is, _ := s.Get(ctx, dev, "KYB-1"); is.Status() != domain.StatusTodo {
		t.Fatal("rejected transition must not persist")
	}

	is, err := s.Transition(ctx, dev, "KYB-1", "in_progress")
	if err != nil || is.Status() != domain.StatusInProgress {
		t.Fatalf("Transition = %+v, %v", is, err)
	}
	if stored, _ := s.Get(ctx, dev, "KYB-1"); stored.Status() != domain.StatusInProgress {
		t.Fatal("transition must persist")
	}
	ev, ok := rec.all()[0].(domain.IssueTransitioned)
	if len(rec.all()) != 1 || !ok || ev.From != domain.StatusTodo || ev.To != domain.StatusInProgress {
		t.Fatalf("events = %+v", rec.all())
	}
	if _, err := s.Transition(ctx, dev, "KYB-9", "done"); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestListIssues(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	for _, title := range []string{"a", "b", "c"} {
		_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: title, Type: "task"})
	}
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "OPS", Title: "z", Type: "task"})
	_, _ = s.Transition(ctx, dev, "KYB-2", "in_progress")

	all, _ := s.List(ctx, dev, "KYB", app.ListQuery{})
	if len(all) != 3 || all[0].Key().Number() != 1 || all[2].Key().Number() != 3 {
		t.Fatalf("List = %v", all)
	}
	inProg, _ := s.List(ctx, dev, "KYB", app.ListQuery{Status: "in_progress"})
	if len(inProg) != 1 || inProg[0].Key().String() != "KYB-2" {
		t.Fatalf("filtered = %v", inProg)
	}
}

func TestAccessControl(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	_, _ = s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})

	if _, err := s.Get(ctx, viewer, "KYB-1"); err != nil {
		t.Fatalf("viewer read: %v", err)
	}
	if _, err := s.List(ctx, viewer, "KYB", app.ListQuery{}); err != nil {
		t.Fatalf("viewer list: %v", err)
	}
	if _, err := s.Create(ctx, viewer, app.CreateIssue{Project: "KYB", Title: "x", Type: "task"}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer create err = %v", err)
	}
	if _, err := s.Transition(ctx, viewer, "KYB-1", "in_progress"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("viewer transition err = %v", err)
	}
	// Outsiders cannot learn whether a project or issue exists.
	for _, err := range []error{
		func() error { _, err := s.Get(ctx, alien, "KYB-1"); return err }(),
		func() error { _, err := s.List(ctx, alien, "KYB", app.ListQuery{}); return err }(),
		func() error { _, err := s.Transition(ctx, alien, "KYB-1", "done"); return err }(),
		func() error {
			_, err := s.Create(ctx, alien, app.CreateIssue{Project: "KYB", Title: "x", Type: "task"})
			return err
		}(),
	} {
		if !errors.Is(err, app.ErrProjectNotFound) {
			t.Fatalf("outsider err = %v, want ErrProjectNotFound", err)
		}
	}
}

func (f fakeAccess) IsAssignable(_ context.Context, project, user string) (bool, error) {
	return f[project][user] == "w", nil
}

// QA parity: like Jira's Assignable User permission, viewers cannot be assignees.
func TestViewerIsNotAssignable(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	is, _ := s.Create(ctx, dev, app.CreateIssue{Project: "KYB", Title: "Login", Type: "task"})
	if _, err := s.Edit(ctx, dev, "KYB-1", app.EditIssue{Version: is.Version(), AssigneeSet: true, Assignee: viewer}); !errors.Is(err, app.ErrInvalidAssignee) {
		t.Fatalf("viewer assignee err = %v", err)
	}
}
