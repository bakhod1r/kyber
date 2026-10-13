package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/application"
	"github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/calendar/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var errDown = errors.New("db down")

// organizers: lead invites KYB and the workspace; dev may not; "BROKEN" fails.
type organizers struct{}

func (organizers) InviteProject(_ context.Context, actor, project string) error {
	switch {
	case project != "KYB":
		return application.ErrProjectNotFound
	case actor != "lead":
		return application.ErrForbidden
	}
	return nil
}

func (organizers) InviteWorkspace(_ context.Context, actor string) error {
	if actor != "lead" {
		return application.ErrForbidden
	}
	return nil
}

type audience struct{ fail bool }

func (a audience) ProjectMembers(context.Context, string) ([]string, error) {
	if a.fail {
		return nil, errDown
	}
	return []string{"dev", "lead"}, nil
}

func (a audience) WorkspaceMembers(context.Context) ([]string, error) {
	if a.fail {
		return nil, errDown
	}
	return []string{"dev", "lead", "qa"}, nil
}

func teamSetup(aud audience) (*application.Service, *memory.Repository, *time.Time) {
	repo := memory.NewRepository()
	now := t0
	n := 0
	s := application.NewService(repo, members{"w-1/lead": true, "w-1/dev": true, "w-1/qa": true},
		func() string { n++; return fmt.Sprintf("m-%d", n) }, func() time.Time { return now }).
		WithTeams(organizers{}, aud)
	return s, repo, &now
}

func TestTeamEvents(t *testing.T) {
	s, repo, _ := teamSetup(audience{})
	ctx := tenant.With(context.Background(), "w-1")
	slot := application.Schedule{Title: "Sprint review", Start: t0.Add(time.Hour), End: t0.Add(2 * time.Hour)}

	p := slot
	p.Scope, p.Project = "project", "KYB"
	m, err := s.Schedule(ctx, "lead", p)
	if err != nil || len(m.Attendees) != 2 || m.Audience.Project != "KYB" {
		t.Fatalf("project event = %+v %v", m, err)
	}
	w := slot
	w.Scope = "workspace"
	if m, err := s.Schedule(ctx, "lead", w); err != nil || len(m.Attendees) != 3 || m.Audience.Scope != domain.ScopeWorkspace {
		t.Fatalf("workspace event = %+v %v", m, err)
	}
	if len(repo.Outbox()) != 2 {
		t.Fatalf("outbox = %v", repo.Outbox())
	}
	for _, tc := range []struct {
		actor string
		in    application.Schedule
		want  error
	}{
		{"dev", p, application.ErrForbidden},
		{"dev", w, application.ErrForbidden},
		{"lead", application.Schedule{Title: "x", Start: slot.Start, End: slot.End, Scope: "project", Project: "OPS"}, application.ErrProjectNotFound},
	} {
		if _, err := s.Schedule(ctx, tc.actor, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s %+v: %v", tc.actor, tc.in, err)
		}
	}
	broken, _, _ := teamSetup(audience{fail: true})
	for _, in := range []application.Schedule{p, w} {
		if _, err := broken.Schedule(ctx, "lead", in); !errors.Is(err, errDown) {
			t.Errorf("audience failure %s: %v", in.Scope, err)
		}
	}

	// RSVP
	id := string(m.ID)
	if got, err := s.Respond(ctx, "dev", id, "going"); err != nil || got.Responses["dev"] != domain.Going {
		t.Fatalf("respond = %+v %v", got, err)
	}
	if again, _ := repo.ByID(ctx, m.ID); again.Responses["dev"] != domain.Going {
		t.Fatalf("stored = %v", again.Responses)
	}
	for _, tc := range []struct {
		ctx          context.Context
		actor, id, a string
		want         error
	}{
		{ctx, "dev", id, "perhaps", domain.ErrInvalidResponse},
		{ctx, "qa", id, "going", domain.ErrMeetingNotFound}, // not invited
		{ctx, "dev", "m-404", "going", domain.ErrMeetingNotFound},
		{tenant.With(context.Background(), "w-2"), "dev", id, "going", domain.ErrMeetingNotFound},
	} {
		if _, err := s.Respond(tc.ctx, tc.actor, tc.id, tc.a); !errors.Is(err, tc.want) {
			t.Errorf("respond %s %s: %v", tc.actor, tc.a, err)
		}
	}
}

func TestRemindDue(t *testing.T) {
	s, repo, now := teamSetup(audience{})
	ctx := tenant.With(context.Background(), "w-1")
	soon, _ := s.Schedule(ctx, "lead", application.Schedule{Title: "Soon", Start: t0.Add(30 * time.Minute), End: t0.Add(time.Hour), Attendees: []string{"dev"}})
	_, _ = s.Schedule(ctx, "lead", application.Schedule{Title: "Later", Start: t0.Add(3 * time.Hour), End: t0.Add(4 * time.Hour)})
	// Starting exactly when the job runs: too late for a reminder.
	_, _ = s.Schedule(ctx, "lead", application.Schedule{Title: "Now", Start: t0.Add(20 * time.Minute), End: t0.Add(time.Hour)})

	if n, err := s.RemindDue(context.Background()); err != nil || n != 0 {
		t.Fatalf("too early: %d %v", n, err)
	}
	*now = t0.Add(20 * time.Minute)
	if n, err := s.RemindDue(context.Background()); err != nil || n != 1 {
		t.Fatalf("due: %d %v", n, err)
	}
	if n, _ := s.RemindDue(context.Background()); n != 0 {
		t.Fatalf("reminded twice: %d", n)
	}
	events := repo.Outbox()
	if r, ok := events[len(events)-1].(domain.Reminder); !ok || r.ID != soon.ID {
		t.Fatalf("last event = %+v", events[len(events)-1])
	}
	if meta := repo.OutboxMeta(); meta[len(meta)-1].Workspace != "w-1" {
		t.Fatalf("reminder recorded outside its workspace: %+v", meta[len(meta)-1])
	}
}

// racing finds a due meeting but loses the reminder to another replica, or fails.
type racing struct {
	*memory.Repository
	err error
}

func (r racing) SaveReminder(context.Context, *domain.Meeting) error { return r.err }

type blind struct{ *memory.Repository }

func (blind) DueForReminder(context.Context, time.Time, time.Time) ([]*domain.Meeting, error) {
	return nil, errDown
}

func TestRemindDueFailures(t *testing.T) {
	ctx := tenant.With(context.Background(), "w-1")
	for _, tc := range []struct {
		err      error
		wantSent int
		wantErr  error
	}{
		{domain.ErrAlreadyReminded, 0, nil},
		{errDown, 0, errDown},
	} {
		repo := memory.NewRepository()
		mk := application.NewService(repo, members{"w-1/lead": true}, func() string { return "m-1" }, func() time.Time { return t0 })
		_, _ = mk.Schedule(ctx, "lead", application.Schedule{Title: "Soon", Start: t0.Add(5 * time.Minute), End: t0.Add(time.Hour)})
		s := application.NewService(racing{repo, tc.err}, members{}, nil, func() time.Time { return t0 })
		if n, err := s.RemindDue(ctx); n != tc.wantSent || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
			t.Errorf("%v: %d %v", tc.err, n, err)
		}
	}
	s := application.NewService(blind{memory.NewRepository()}, members{}, nil, func() time.Time { return t0 })
	if _, err := s.RemindDue(ctx); !errors.Is(err, errDown) {
		t.Fatalf("listing failure: %v", err)
	}
}
