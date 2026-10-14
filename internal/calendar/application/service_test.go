package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/application"
	"github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/calendar/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type members map[string]bool // "workspace/user"

func (m members) IsMember(_ context.Context, ws, u string) (bool, error) {
	if u == "broken" {
		return false, errors.New("db down")
	}
	return m[ws+"/"+u], nil
}

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func setup() (*application.Service, context.Context) {
	n := 0
	s := application.NewService(memory.NewRepository(), members{"w-1/lead": true, "w-1/dev": true, "w-2/dev": true},
		func() string { n++; return fmt.Sprintf("m-%d", n) }, func() time.Time { return t0 })
	return s, tenant.With(context.Background(), "w-1")
}

func TestScheduleAndTimetable(t *testing.T) {
	s, ctx := setup()
	m, err := s.Schedule(ctx, "lead", application.Schedule{Title: "Standup", Start: t0.Add(time.Hour), End: t0.Add(75 * time.Minute), Attendees: []string{"dev"}})
	if err != nil || m.Workspace != "w-1" || len(m.Attendees) != 2 {
		t.Fatalf("schedule = %+v %v", m, err)
	}
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: "x", Start: t0, End: t0.Add(time.Hour), Attendees: []string{"stranger"}}); !errors.Is(err, application.ErrNotMember) {
		t.Fatalf("stranger err = %v", err)
	}
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: " ", Start: t0, End: t0.Add(time.Hour)}); !errors.Is(err, domain.ErrEmptyTitle) {
		t.Fatalf("blank err = %v", err)
	}
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: "x", Start: t0, End: t0.Add(time.Hour), Attendees: []string{"broken"}}); err == nil {
		t.Fatal("membership errors surface")
	}
	day, err := s.Timetable(ctx, "dev", t0, t0.Add(24*time.Hour))
	if err != nil || len(day) != 1 || day[0].Title != "Standup" {
		t.Fatalf("timetable = %v %v", day, err)
	}
	// The same user in another workspace does not see it on that workspace's timetable…
	if other, _ := s.Timetable(tenant.With(context.Background(), "w-2"), "dev", t0, t0.Add(24*time.Hour)); len(other) != 0 {
		t.Fatalf("w-2 timetable = %v", other)
	}
	// …but is busy everywhere (focus is blocked in every workspace).
	busy, err := s.BusyFor(context.Background(), "dev", t0, t0.Add(24*time.Hour))
	if err != nil || len(busy) != 1 {
		t.Fatalf("busy = %v %v", busy, err)
	}
	if _, err := s.Timetable(ctx, "dev", t0, t0.Add(63*24*time.Hour)); !errors.Is(err, application.ErrRange) {
		t.Fatalf("range err = %v", err)
	}
	if _, err := s.Timetable(ctx, "dev", t0, t0); !errors.Is(err, application.ErrRange) {
		t.Fatalf("empty range err = %v", err)
	}
}

func TestCancel(t *testing.T) {
	s, ctx := setup()
	m, _ := s.Schedule(ctx, "lead", application.Schedule{Title: "Retro", Start: t0, End: t0.Add(time.Hour), Attendees: []string{"dev"}})
	if err := s.Cancel(ctx, "stranger", string(m.ID)); !errors.Is(err, domain.ErrMeetingNotFound) {
		t.Fatalf("outsider cancel err = %v (no existence leak)", err)
	}
	if err := s.Cancel(ctx, "dev", string(m.ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("attendee cancel err = %v", err)
	}
	if err := s.Cancel(tenant.With(context.Background(), "w-2"), "lead", string(m.ID)); !errors.Is(err, domain.ErrMeetingNotFound) {
		t.Fatalf("other workspace cancel err = %v", err)
	}
	if err := s.Cancel(ctx, "lead", "nope"); !errors.Is(err, domain.ErrMeetingNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if err := s.Cancel(ctx, "lead", string(m.ID)); err != nil {
		t.Fatal(err)
	}
	if day, _ := s.Timetable(ctx, "dev", t0, t0.Add(time.Hour)); len(day) != 0 {
		t.Fatalf("after cancel = %v", day)
	}
	// Without a tenant (single-tenant internal calls) the default workspace applies.
	if _, err := s.Schedule(context.Background(), "lead", application.Schedule{Title: "x", Start: t0, End: t0.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

type brokenRepo struct{ domain.Repository }

func (brokenRepo) ForUser(context.Context, domain.UserID, time.Time, time.Time) ([]*domain.Meeting, error) {
	return nil, errors.New("db down")
}

func TestTimetableStorageError(t *testing.T) {
	s := application.NewService(brokenRepo{memory.NewRepository()}, members{}, func() string { return "m" }, time.Now)
	if _, err := s.Timetable(context.Background(), "u", t0, t0.Add(time.Hour)); err == nil {
		t.Fatal("storage errors surface")
	}
}

func TestNoMeetingsInThePast(t *testing.T) {
	s, ctx := setup()
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: "Old", Start: t0.Add(-time.Hour), End: t0}); !errors.Is(err, application.ErrInPast) {
		t.Fatalf("err = %v", err)
	}
}

// readable is a fake Conversations: which conversations each user may read.
type readable map[string][]string

func (r readable) Readable(_ context.Context, user, conv string) (bool, error) {
	if conv == "broken" {
		return false, errors.New("chat down")
	}
	return slices.Contains(r[user], conv), nil
}

// S76: a meeting can happen in a channel's huddle the organizer may read.
func TestMeetingInAHuddle(t *testing.T) {
	s, ctx := setup()
	s = s.WithConversations(readable{"lead": {"c-1"}})
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: "Sync", Start: t0.Add(time.Hour), End: t0.Add(2 * time.Hour), Conversation: "c-2"}); !errors.Is(err, application.ErrNoConversation) {
		t.Fatalf("unreadable channel = %v", err)
	}
	m, err := s.Schedule(ctx, "lead", application.Schedule{Title: "Sync", Start: t0.Add(time.Hour), End: t0.Add(2 * time.Hour), Conversation: "c-1"})
	if err != nil || m.Conversation != "c-1" {
		t.Fatalf("meeting = %+v %v", m, err)
	}
	if _, err := s.Schedule(ctx, "lead", application.Schedule{Title: "Sync", Start: t0.Add(time.Hour), End: t0.Add(2 * time.Hour), Conversation: "broken"}); err == nil || errors.Is(err, application.ErrNoConversation) {
		t.Fatalf("a failing check must surface: %v", err)
	}
}

func TestReschedule(t *testing.T) {
	s, ctx := setup()
	m, _ := s.Schedule(ctx, "lead", application.Schedule{Title: "Retro", Start: t0.Add(time.Hour), End: t0.Add(2 * time.Hour), Attendees: []string{"dev"}})
	in := application.Change{Title: "Retro v2", Start: t0.Add(3 * time.Hour), End: t0.Add(4 * time.Hour)}
	for _, tc := range []struct {
		ctx       context.Context
		actor, id string
		in        application.Change
		want      error
	}{
		{ctx, "stranger", string(m.ID), in, domain.ErrMeetingNotFound},
		{ctx, "dev", string(m.ID), in, application.ErrForbidden},
		{tenant.With(context.Background(), "w-2"), "lead", string(m.ID), in, domain.ErrMeetingNotFound},
		{ctx, "lead", "nope", in, domain.ErrMeetingNotFound},
		{ctx, "lead", string(m.ID), application.Change{Title: "x", Start: t0.Add(-time.Hour), End: t0}, application.ErrInPast},
		{ctx, "lead", string(m.ID), application.Change{Title: "", Start: t0, End: t0.Add(time.Hour)}, domain.ErrEmptyTitle},
	} {
		if _, err := s.Reschedule(tc.ctx, tc.actor, tc.id, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.actor, err, tc.want)
		}
	}
	got, err := s.Reschedule(ctx, "lead", string(m.ID), in)
	if err != nil || got.Title != "Retro v2" || !got.Start.Equal(in.Start) {
		t.Fatalf("reschedule = %+v %v", got, err)
	}
	if day, _ := s.Timetable(ctx, "dev", t0, t0.Add(24*time.Hour)); len(day) != 1 || !day[0].End.Equal(in.End) {
		t.Fatalf("timetable = %v", day)
	}
}
