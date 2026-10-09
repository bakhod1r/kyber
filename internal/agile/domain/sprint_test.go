package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/agile/domain"
)

var t0 = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

func newSprint(t *testing.T) *domain.Sprint {
	t.Helper()
	s, err := domain.NewSprint("s-1", "KYB", "  Sprint 1 ", " Ship the board ")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewSprint(t *testing.T) {
	s := newSprint(t)
	if s.Name() != "Sprint 1" || s.Goal() != "Ship the board" || s.State() != domain.StatePlanned || s.Project() != "KYB" || s.ID() != "s-1" {
		t.Fatalf("sprint = %+v", s)
	}
	if ev := s.PullEvents(); len(ev) != 1 || ev[0].EventName() != "sprint.created" {
		t.Fatalf("events = %+v", ev)
	}
	for _, bad := range []struct{ name, goal string }{{"  ", ""}, {strings.Repeat("x", 101), ""}, {"ok", strings.Repeat("g", 2001)}} {
		if _, err := domain.NewSprint("s-2", "KYB", bad.name, bad.goal); !errors.Is(err, domain.ErrInvalidSprint) {
			t.Fatalf("NewSprint(%q) err = %v", bad.name, err)
		}
	}
}

func TestSprintLifecycle(t *testing.T) {
	s := newSprint(t)
	s.PullEvents()
	if err := s.Complete(t0); !errors.Is(err, domain.ErrSprintState) {
		t.Fatalf("complete planned err = %v", err)
	}
	if err := s.Start(t0); err != nil {
		t.Fatal(err)
	}
	if s.State() != domain.StateActive || !s.StartedAt().Equal(t0) {
		t.Fatalf("after start: %q %v", s.State(), s.StartedAt())
	}
	if err := s.Start(t0); !errors.Is(err, domain.ErrSprintState) {
		t.Fatalf("double start err = %v", err)
	}
	if err := s.Complete(t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if s.State() != domain.StateClosed || !s.CompletedAt().Equal(t0.Add(time.Hour)) {
		t.Fatalf("after complete: %q %v", s.State(), s.CompletedAt())
	}
	names := []string{}
	for _, e := range s.PullEvents() {
		names = append(names, e.EventName())
	}
	if strings.Join(names, ",") != "sprint.started,sprint.completed" {
		t.Fatalf("events = %v", names)
	}
	if s.CanHoldIssues() {
		t.Fatal("closed sprints cannot hold issues")
	}
}

func TestCanHoldIssues(t *testing.T) {
	s := newSprint(t)
	if !s.CanHoldIssues() {
		t.Fatal("planned sprint must accept issues")
	}
	_ = s.Start(t0)
	if !s.CanHoldIssues() {
		t.Fatal("active sprint must accept issues")
	}
}

func TestRehydrate(t *testing.T) {
	s := domain.Rehydrate(domain.Snapshot{ID: "s-9", Project: "KYB", Name: "n", Goal: "g", State: domain.StateActive, StartedAt: t0})
	if s.State() != domain.StateActive || !s.StartedAt().Equal(t0) || len(s.PullEvents()) != 0 {
		t.Fatalf("rehydrated = %+v", s)
	}
}

func TestVersion(t *testing.T) {
	s := newSprint(t)
	if s.Version() != 0 {
		t.Fatal("new sprint must be unpersisted")
	}
	s.MarkPersisted()
	if s.Version() != 1 || domain.Rehydrate(domain.Snapshot{Version: 7}).Version() != 7 {
		t.Fatal("version not tracked")
	}
}
