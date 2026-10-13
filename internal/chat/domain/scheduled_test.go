package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestScheduledMessage(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	if _, err := domain.NewScheduled("s-1", "ws", "c-1", "u-1", "hi", t0, t0); !errors.Is(err, domain.ErrInvalidSchedule) {
		t.Fatalf("now = %v", err)
	}
	if _, err := domain.NewScheduled("s-1", "ws", "c-1", "u-1", "hi", t0.Add(121*24*time.Hour), t0); !errors.Is(err, domain.ErrInvalidSchedule) {
		t.Fatalf("too far = %v", err)
	}
	if _, err := domain.NewScheduled("s-1", "ws", "c-1", "u-1", " ", t0.Add(time.Hour), t0); !errors.Is(err, domain.ErrInvalidBody) {
		t.Fatalf("empty = %v", err)
	}
	s, err := domain.NewScheduled("s-1", "ws", "c-1", "u-1", " standup notes ", t0.Add(time.Hour), t0)
	if err != nil || s.Body != "standup notes" {
		t.Fatalf("scheduled = %+v %v", s, err)
	}
	if err := s.Reschedule("updated notes", t0.Add(2*time.Hour), t0); err != nil || s.Body != "updated notes" {
		t.Fatalf("reschedule = %+v %v", s, err)
	}
	m := s.Message("m-1", t0.Add(2*time.Hour))
	if m.Author != "u-1" || m.Conversation != "c-1" || m.Body != "updated notes" || m.Ref != "scheduled:s-1" || len(m.PullEvents()) != 1 {
		t.Fatalf("message = %+v", m)
	}
}

func TestPersonalReminder(t *testing.T) {
	t0 := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	r, err := domain.NewReminder("r-1", "ws", "u-1", "c-1", "call the bank", t0.Add(time.Hour), t0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Remind(t0) || !r.Remind(t0.Add(time.Hour)) || r.Remind(t0.Add(2*time.Hour)) {
		t.Fatal("remind once at its time")
	}
	if ev := r.PullEvents(); len(ev) != 1 || ev[0].User != "u-1" || ev[0].Excerpt != "call the bank" || ev[0].Message != "" {
		t.Fatalf("events = %+v", ev)
	}
	if _, err := domain.NewReminder("r-2", "ws", "u-1", "c-1", "x", t0, t0); !errors.Is(err, domain.ErrInvalidReminder) {
		t.Fatalf("past = %v", err)
	}
}
