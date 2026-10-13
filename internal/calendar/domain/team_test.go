package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
)

func TestTeamEventAnnouncesItself(t *testing.T) {
	m, err := domain.PlanMeeting("m-1", "w-1", "Sprint review", t0, t0.Add(time.Hour), "u-lead", []domain.UserID{"u-a", "u-b"},
		domain.Audience{Scope: domain.ScopeProject, Project: "KYB"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Audience.Project != "KYB" || len(m.Attendees) != 3 {
		t.Fatalf("meeting = %+v", m)
	}
	evs := m.PullEvents()
	s, ok := evs[0].(domain.Scheduled)
	if len(evs) != 1 || !ok || s.EventName() != "meeting.scheduled" || s.Project != "KYB" || s.Organizer != "u-lead" || len(s.Attendees) != 3 || !s.Start.Equal(t0) {
		t.Fatalf("events = %+v", evs)
	}
	if len(m.PullEvents()) != 0 {
		t.Fatal("events are pulled once")
	}
	if _, err := domain.PlanMeeting("m", "w", "x", t0, t0.Add(time.Hour), "u", nil, domain.Audience{Scope: domain.ScopeProject}); !errors.Is(err, domain.ErrInvalidAudience) {
		t.Fatalf("project scope without project: %v", err)
	}
	if _, err := domain.PlanMeeting("m", "w", "x", t0, t0.Add(time.Hour), "u", nil, domain.Audience{Scope: "everyone"}); !errors.Is(err, domain.ErrInvalidAudience) {
		t.Fatalf("unknown scope: %v", err)
	}
	if _, err := domain.PlanMeeting("m", "w", "", t0, t0.Add(time.Hour), "u", nil, domain.Audience{}); !errors.Is(err, domain.ErrEmptyTitle) {
		t.Fatalf("invalid meeting: %v", err)
	}
	if p, _ := domain.PlanMeeting("m-3", "w-1", "Chat", t0, t0.Add(time.Hour), "u-a", nil, domain.Audience{}); p.Audience.Scope != domain.ScopePeople {
		t.Fatalf("no scope means chosen people: %+v", p.Audience)
	}
	if p, _ := domain.NewMeeting("m-2", "w-1", "1:1", t0, t0.Add(time.Hour), "u-a", nil); p.Audience.Scope != domain.ScopePeople {
		t.Fatalf("a plain meeting invites people: %+v", p.Audience)
	}
}

func TestRespond(t *testing.T) {
	m, _ := domain.NewMeeting("m-1", "w-1", "Review", t0, t0.Add(time.Hour), "u-lead", []domain.UserID{"u-a"})
	if err := m.Respond("u-a", domain.Declined); err != nil || m.Responses["u-a"] != domain.Declined {
		t.Fatalf("respond = %v %v", err, m.Responses)
	}
	if err := m.Respond("u-out", domain.Going); !errors.Is(err, domain.ErrNotAttendee) {
		t.Fatalf("outsider: %v", err)
	}
	if err := m.Respond("u-a", "perhaps"); !errors.Is(err, domain.ErrInvalidResponse) {
		t.Fatalf("unknown answer: %v", err)
	}
	for _, s := range []string{"going", "maybe", "declined"} {
		if r, err := domain.ParseResponse(s); err != nil || string(r) != s {
			t.Errorf("%s: %v", s, err)
		}
	}
}

func TestCancelAndRemind(t *testing.T) {
	m, _ := domain.PlanMeeting("m-1", "w-1", "Review", t0, t0.Add(time.Hour), "u-lead", []domain.UserID{"u-a", "u-b"}, domain.Audience{Scope: domain.ScopeWorkspace})
	m.PullEvents()
	_ = m.Respond("u-b", domain.Declined)

	if m.Remind(t0.Add(-domain.ReminderLead - time.Minute)) {
		t.Fatal("too early to remind")
	}
	if !m.Remind(t0.Add(-10*time.Minute)) || !m.Reminded {
		t.Fatal("reminder due")
	}
	r := m.PullEvents()[0].(domain.Reminder)
	if r.EventName() != "meeting.reminder" || len(r.Attendees) != 2 || r.Attendees[0] != "u-a" || r.Attendees[1] != "u-lead" {
		t.Fatalf("reminder = %+v (declined attendees are not reminded)", r)
	}
	if m.Remind(t0.Add(-5 * time.Minute)) {
		t.Fatal("reminded twice")
	}
	late, _ := domain.NewMeeting("m-2", "w-1", "Late", t0, t0.Add(time.Hour), "u-lead", nil)
	if late.Remind(t0.Add(time.Minute)) {
		t.Fatal("no reminder once it started")
	}

	m.Cancel("u-lead")
	c := m.PullEvents()[0].(domain.Cancelled)
	if c.EventName() != "meeting.cancelled" || c.By != "u-lead" || len(c.Attendees) != 3 || c.Title != "Review" {
		t.Fatalf("cancelled = %+v", c)
	}
}
