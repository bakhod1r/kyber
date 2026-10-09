package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/adapter/memory"
	"github.com/bakhod1r/kyber/internal/insights/app"
	"github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

type fakes struct{}

func (fakes) Authorize(_ context.Context, actor, project string) error {
	if actor != "dev" || project != "KYB" {
		return app.ErrProjectNotFound
	}
	return nil
}

func (fakes) Issues(_ context.Context, project string) ([]app.IssueRow, error) {
	five, three := 50, 30
	return []app.IssueRow{
		{Key: "KYB-1", Type: "bug", Status: "todo", Priority: "high", Assignee: "u-a", Points: &five},
		{Key: "KYB-2", Type: "task", Status: "done", Priority: "medium", Assignee: "u-a", Points: &three},
		{Key: "KYB-3", Type: "task", Status: "in_progress", Priority: "medium"},
	}, nil
}

func (fakes) DisplayName(_ context.Context, id string) (string, error) {
	return map[string]string{"u-a": "Ali"}[id], nil
}

var t0 = time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)

func (fakes) Sprint(_ context.Context, id string) (app.SprintInfo, error) {
	if id != "s-1" {
		return app.SprintInfo{}, app.ErrSprintNotFound
	}
	return app.SprintInfo{ID: "s-1", Project: "KYB", Name: "S1", State: "active", Start: t0, End: t0.Add(72 * time.Hour)}, nil
}

func (fakes) ClosedSprints(_ context.Context, project string, limit int) ([]app.SprintInfo, error) {
	return []app.SprintInfo{{ID: "s-0", Project: "KYB", Name: "S0", State: "closed", Start: t0.Add(-96 * time.Hour), End: t0.Add(-24 * time.Hour), Completed: t0.Add(-24 * time.Hour)}}, nil
}

func setup() (*app.Service, *memory.Repository) {
	repo := memory.NewRepository()
	f := fakes{}
	return app.NewService(app.Deps{Repo: repo, Access: f, Issues: f, Users: f, Sprints: f, Now: func() time.Time { return t0.Add(30 * time.Hour) }}), repo
}

func m(id int64, name, payload string, at time.Time) outbox.Message {
	return outbox.Message{ID: id, Name: name, Payload: []byte(payload), At: at}
}

func TestIngest(t *testing.T) {
	ctx := context.Background()
	s, repo := setup()
	id := `"id":"i-1","key":"KYB-12"`
	for _, msg := range []outbox.Message{
		m(1, "issue.created", `{`+id+`,"type":"task","title":"t"}`, t0),
		m(2, "issue.estimated", `{`+id+`,"from":null,"to":2.5}`, t0.Add(time.Minute)),
		m(3, "issue.sprint_changed", `{`+id+`,"from":"","to":"s-1"}`, t0.Add(2*time.Minute)),
		m(4, "issue.transitioned", `{`+id+`,"from":"todo","to":"in_progress"}`, t0.Add(3*time.Minute)),
		m(4, "issue.transitioned", `{`+id+`,"from":"todo","to":"in_progress"}`, t0.Add(3*time.Minute)), // redelivery
		m(5, "issue.estimated", `garbage`, t0),
	} {
		if err := s.OnIssueEvent(ctx, msg); err != nil {
			t.Fatalf("event %d: %v", msg.ID, err)
		}
	}
	got, _ := repo.ForProject(ctx, "KYB")
	if len(got) != 4 {
		t.Fatalf("entries = %+v", got)
	}
	if got[0].Kind != domain.KindCreated || got[0].Issue != "i-1" || got[0].To != "todo" || !got[0].At.Equal(t0) {
		t.Fatalf("created = %+v", got[0])
	}
	if got[1].Kind != domain.KindEstimate || got[1].ToPoints == nil || *got[1].ToPoints != 25 || got[1].FromPoints != nil {
		t.Fatalf("estimate = %+v", got[1])
	}
	if got[2].To != "s-1" || got[3].To != "in_progress" {
		t.Fatalf("sprint/status = %+v %+v", got[2], got[3])
	}
}

func TestSummary(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	sum, err := s.Summary(ctx, "dev", "KYB")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 3 || sum.Open != 2 || sum.Done != 1 || sum.Unassigned != 1 || sum.TotalPoints != 80 || sum.OpenPoints != 50 {
		t.Fatalf("totals = %+v", sum)
	}
	if len(sum.ByStatus) != 3 || sum.ByStatus[0].Key != "todo" || sum.ByStatus[2].Key != "done" || sum.ByStatus[2].Points != 30 {
		t.Fatalf("by status (workflow order) = %+v", sum.ByStatus)
	}
	if len(sum.Workload) != 2 || sum.Workload[0].Name != "Ali" || sum.Workload[0].Count != 1 || sum.Workload[0].Points != 50 || sum.Workload[1].Name != "Unassigned" {
		t.Fatalf("workload (open issues only) = %+v", sum.Workload)
	}
	if _, err := s.Summary(ctx, "stranger", "KYB"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
}

func TestBurndownAndVelocityUseSprintWindows(t *testing.T) {
	ctx := context.Background()
	s, _ := setup()
	bd, err := s.Burndown(ctx, "dev", "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if bd.Sprint.Name != "S1" || !bd.Report.Ideal[1].At.Equal(t0.Add(72*time.Hour)) || !bd.Report.Samples[len(bd.Report.Samples)-1].At.Equal(t0.Add(30*time.Hour)) {
		t.Fatalf("burndown window = %+v", bd)
	}
	if _, err := s.Burndown(ctx, "stranger", "s-1"); !errors.Is(err, app.ErrSprintNotFound) {
		t.Fatalf("outsider err = %v", err)
	}
	v, err := s.Velocity(ctx, "dev", "KYB")
	if err != nil || len(v) != 1 || v[0].Name != "S0" {
		t.Fatalf("velocity = %+v, %v", v, err)
	}
}
