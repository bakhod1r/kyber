package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestWorkflow(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC) // a Tuesday
	w, err := domain.NewWorkflow("w-1", "ws", "c-1", domain.OnJoin, 0, 0, "Welcome {name}!", "u-1", t0)
	if err != nil || w.Welcome("Ali") != "Welcome Ali!" {
		t.Fatalf("join = %+v %v", w, err)
	}
	// Every weekday at 09:30 UTC.
	s, err := domain.NewWorkflow("w-2", "ws", "c-1", domain.OnSchedule, 9*60+30, domain.Weekdays, "Standup time", "u-1", t0)
	if err != nil {
		t.Fatal(err)
	}
	if due := s.Due(t0); !due.IsZero() {
		t.Fatalf("due before 09:30 = %v", due)
	}
	at := s.Due(t0.Add(2 * time.Hour))
	if !at.Equal(time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("due = %v", at)
	}
	s.Ran(at)
	if due := s.Due(t0.Add(3 * time.Hour)); !due.IsZero() {
		t.Fatalf("ran twice the same day: %v", due)
	}
	sat := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if due := s.Due(sat); !due.Equal(time.Date(2026, 10, 16, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("Saturday: the latest occurrence is Friday's = %v", due)
	}
	for _, bad := range []struct {
		trig domain.Trigger
		min  int
		days domain.Days
		text string
	}{
		{domain.OnSchedule, 24 * 60, domain.Weekdays, "x"},
		{domain.OnSchedule, 60, 0, "x"},
		{domain.OnJoin, 0, 0, " "},
		{"cron", 0, 0, "x"},
	} {
		if _, err := domain.NewWorkflow("w", "ws", "c", bad.trig, bad.min, bad.days, bad.text, "u", t0); !errors.Is(err, domain.ErrInvalidWorkflow) && !errors.Is(err, domain.ErrInvalidBody) {
			t.Fatalf("%+v = %v", bad, err)
		}
	}
}
