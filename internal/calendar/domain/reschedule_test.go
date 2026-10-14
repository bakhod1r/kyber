package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
)

func TestReschedule(t *testing.T) {
	m, _ := domain.NewMeeting("m-1", "w-1", "Review", t0, t0.Add(time.Hour), "u-lead", []domain.UserID{"u-a"})
	m.PullEvents()
	m.Remind(t0.Add(-5 * time.Minute))
	m.PullEvents()
	for _, tc := range []struct {
		title      string
		start, end time.Time
		want       error
	}{
		{" ", t0, t0.Add(time.Hour), domain.ErrEmptyTitle},
		{"x", t0, t0, domain.ErrInvalidTime},
		{"x", time.Time{}, t0, domain.ErrInvalidTime},
		{"x", t0, t0.Add(9 * time.Hour), domain.ErrInvalidTime},
	} {
		if err := m.Reschedule(tc.title, tc.start, tc.end); !errors.Is(err, tc.want) {
			t.Errorf("%q: %v", tc.title, err)
		}
	}
	if m.Title != "Review" || !m.Start.Equal(t0) || len(m.PullEvents()) != 0 || !m.Reminded {
		t.Fatalf("an invalid change leaves the meeting alone: %+v", m)
	}
	later := time.Date(2026, 10, 13, 14, 0, 0, 0, time.FixedZone("UZT", 5*3600))
	if err := m.Reschedule(" Sprint review ", later, later.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if m.Title != "Sprint review" || m.Start != later.UTC() || m.End != later.Add(30*time.Minute).UTC() || m.Reminded {
		t.Fatalf("rescheduled = %+v", m)
	}
	evs := m.PullEvents()
	r, ok := evs[0].(domain.Rescheduled)
	if len(evs) != 1 || !ok || r.EventName() != "meeting.rescheduled" || r.Organizer != "u-lead" || r.Title != "Sprint review" ||
		!r.Start.Equal(later) || len(r.Attendees) != 2 {
		t.Fatalf("events = %+v", evs)
	}
}
