package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
)

var t0 = time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)

func TestNewMeeting(t *testing.T) {
	m, err := domain.NewMeeting("m-1", "w-1", " Sprint planning ", t0, t0.Add(time.Hour), "u-lead", []domain.UserID{"u-a", "u-lead", "u-a", "u-b"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Sprint planning" || len(m.Attendees) != 3 || m.Attendees[0] != "u-a" || m.Attendees[2] != "u-lead" {
		t.Fatalf("meeting = %+v (organizer attends; duplicates removed; sorted)", m)
	}
	cases := map[string]struct {
		title      string
		start, end time.Time
		want       error
	}{
		"blank title":   {" ", t0, t0.Add(time.Hour), domain.ErrEmptyTitle},
		"ends before":   {"x", t0, t0, domain.ErrInvalidTime},
		"too long":      {"x", t0, t0.Add(domain.MaxDuration + time.Minute), domain.ErrInvalidTime},
		"no start time": {"x", time.Time{}, t0, domain.ErrInvalidTime},
	}
	for name, c := range cases {
		if _, err := domain.NewMeeting("m", "w", c.title, c.start, c.end, "u", nil); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestOverlaps(t *testing.T) {
	m, _ := domain.NewMeeting("m-1", "w-1", "Standup", t0, t0.Add(15*time.Minute), "u-lead", []domain.UserID{"u-a"})
	cases := []struct {
		user     domain.UserID
		from, to time.Time
		want     bool
	}{
		{"u-a", t0.Add(-time.Hour), t0, false},                        // ends exactly at start
		{"u-a", t0.Add(-10 * time.Minute), t0.Add(time.Minute), true}, // crosses the start
		{"u-a", t0.Add(5 * time.Minute), t0.Add(6 * time.Minute), true},
		{"u-a", t0.Add(15 * time.Minute), t0.Add(time.Hour), false}, // starts exactly at end
		{"u-lead", t0, t0.Add(time.Minute), true},
		{"u-other", t0, t0.Add(time.Minute), false}, // not invited
	}
	for _, c := range cases {
		if got := m.Blocks(c.user, c.from, c.to); got != c.want {
			t.Errorf("Blocks(%s, %s..%s) = %v", c.user, c.from.Format("15:04"), c.to.Format("15:04"), got)
		}
	}
}
