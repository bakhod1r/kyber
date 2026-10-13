package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

func TestParseDate(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want domain.Date
		err  bool
	}{
		{"", "", false},
		{" 2026-10-13 ", "2026-10-13", false},
		{"2026-02-29", "", true}, // not a leap year
		{"2026-1-5", "", true},
		{"13.10.2026", "", true},
	} {
		got, err := domain.ParseDate(tc.in)
		if (err != nil) != tc.err || got != tc.want {
			t.Errorf("ParseDate(%q) = %q, %v", tc.in, got, err)
		}
		if err != nil && !errors.Is(err, domain.ErrInvalidSchedule) {
			t.Errorf("ParseDate(%q): %v is not ErrInvalidSchedule", tc.in, err)
		}
	}
}

func TestSchedule(t *testing.T) {
	is := domain.Rehydrate(domain.Snapshot{ID: "i-1", Key: mustKey(t, "KYB-1"), Title: "Login", Type: domain.TypeTask, Status: domain.StatusTodo, Version: 1})
	if s, d := is.Schedule(); s != "" || d != "" {
		t.Fatalf("unscheduled issue: %q %q", s, d)
	}
	if err := is.Plan("2026-10-20", "2026-10-13"); !errors.Is(err, domain.ErrInvalidSchedule) {
		t.Fatalf("start after due: %v", err)
	}
	if err := is.Plan("2026-10-13", "2026-10-13"); err != nil {
		t.Fatalf("one-day schedule: %v", err)
	}
	if err := is.Plan("", "2026-11-01"); err != nil {
		t.Fatalf("due date only: %v", err)
	}
	if s, d := is.Schedule(); s != "" || d != "2026-11-01" {
		t.Fatalf("schedule = %q %q", s, d)
	}
	ch, ok := is.Changelog("u-1")
	if !ok || len(ch.Items) != 1 || ch.Items[0] != (domain.FieldChange{Field: "due_date", To: "2026-11-01"}) {
		t.Fatalf("changelog = %+v", ch.Items)
	}
	if snap := is.Snapshot(); snap.DueDate != "2026-11-01" || snap.StartDate != "" {
		t.Fatalf("snapshot = %q %q", snap.StartDate, snap.DueDate)
	}
}
