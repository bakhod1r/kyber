package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/focus/domain"
)

var t0 = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func start(t *testing.T, busy ...domain.Busy) *domain.Session {
	t.Helper()
	s, err := domain.Start("s-1", "u-a", "w-1", "KYB-7", t0, domain.DefaultFocus, busy)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStartValidation(t *testing.T) {
	s := start(t)
	if s.State != domain.Running || s.Planned != 25*time.Minute || s.Issue != "KYB-7" || !s.StartedAt.Equal(t0) {
		t.Fatalf("session = %+v", s)
	}
	for _, d := range []time.Duration{14 * time.Minute, 61 * time.Minute} {
		if _, err := domain.Start("s", "u", "w", "KYB-1", t0, d, nil); !errors.Is(err, domain.ErrInvalidLength) {
			t.Errorf("%s err = %v", d, err)
		}
	}
	if _, err := domain.Start("s", "u", "w", " ", t0, domain.DefaultFocus, nil); !errors.Is(err, domain.ErrNoIssue) {
		t.Fatalf("no issue err = %v", err)
	}
}

func TestMeetingBlocksStart(t *testing.T) {
	standup := domain.Busy{Title: "Standup", Start: t0.Add(10 * time.Minute), End: t0.Add(25 * time.Minute)}
	// The 25-minute session would run into the standup at 09:10.
	_, err := domain.Start("s", "u", "w", "KYB-1", t0, domain.DefaultFocus, []domain.Busy{standup})
	var conflict *domain.MeetingConflictError
	if !errors.Is(err, domain.ErrMeetingConflict) || !errors.As(err, &conflict) || conflict.Title != "Standup" {
		t.Fatalf("err = %v", err)
	}
	// During the meeting itself.
	if _, err := domain.Start("s", "u", "w", "KYB-1", t0.Add(12*time.Minute), domain.DefaultFocus, []domain.Busy{standup}); !errors.Is(err, domain.ErrMeetingConflict) {
		t.Fatalf("during meeting err = %v", err)
	}
	// Right after it ends is fine.
	if _, err := domain.Start("s", "u", "w", "KYB-1", t0.Add(25*time.Minute), domain.DefaultFocus, []domain.Busy{standup}); err != nil {
		t.Fatalf("after meeting err = %v", err)
	}
	if conflict.Error() == "" {
		t.Fatal("conflict message")
	}
}

func TestPauseResumeComplete(t *testing.T) {
	s := start(t)
	if err := s.Pause(t0.Add(10 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.Pause(t0.Add(11 * time.Minute)); !errors.Is(err, domain.ErrNotRunning) {
		t.Fatalf("double pause err = %v", err)
	}
	if got := s.Focused(t0.Add(20 * time.Minute)); got != 10*time.Minute {
		t.Fatalf("focused while paused = %s", got)
	}
	// Resuming re-checks the calendar for the remaining 15 minutes.
	lunch := domain.Busy{Title: "Lunch talk", Start: t0.Add(30 * time.Minute), End: t0.Add(time.Hour)}
	if err := s.Resume(t0.Add(20*time.Minute), []domain.Busy{lunch}); !errors.Is(err, domain.ErrMeetingConflict) {
		t.Fatalf("resume into meeting err = %v", err)
	}
	if err := s.Resume(t0.Add(20*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Resume(t0.Add(21*time.Minute), nil); !errors.Is(err, domain.ErrNotPaused) {
		t.Fatalf("resume running err = %v", err)
	}
	// 10 focused + 15 more after resuming at 09:20 → completes at 09:35.
	s.Settle(t0.Add(40*time.Minute), nil)
	if s.State != domain.Completed || !s.EndedAt.Equal(t0.Add(35*time.Minute)) || s.Focused(t0.Add(time.Hour)) != 25*time.Minute {
		t.Fatalf("completed = %+v", s)
	}
	if err := s.Stop(t0.Add(time.Hour)); !errors.Is(err, domain.ErrFinished) {
		t.Fatalf("stop finished err = %v", err)
	}
}

func TestStopAndMeetingInterrupts(t *testing.T) {
	s := start(t)
	if err := s.Stop(t0.Add(7 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if s.State != domain.Interrupted || s.Reason != "stopped" || s.Focused(t0.Add(time.Hour)) != 7*time.Minute {
		t.Fatalf("stopped = %+v", s)
	}
	// A meeting scheduled after the session started interrupts it when it begins.
	s2 := start(t)
	sync := domain.Busy{Title: "Incident sync", Start: t0.Add(12 * time.Minute), End: t0.Add(30 * time.Minute)}
	s2.Settle(t0.Add(8*time.Minute), []domain.Busy{sync})
	if s2.State != domain.Running {
		t.Fatal("not yet")
	}
	s2.Settle(t0.Add(20*time.Minute), []domain.Busy{sync})
	if s2.State != domain.Interrupted || s2.Reason != "meeting: Incident sync" || !s2.EndedAt.Equal(t0.Add(12*time.Minute)) {
		t.Fatalf("interrupted = %+v", s2)
	}
	// A paused session is also ended by a meeting, at the meeting start.
	s3 := start(t)
	_ = s3.Pause(t0.Add(5 * time.Minute))
	s3.Settle(t0.Add(20*time.Minute), []domain.Busy{sync})
	if s3.State != domain.Interrupted || s3.Focused(t0.Add(time.Hour)) != 5*time.Minute {
		t.Fatalf("paused + meeting = %+v", s3)
	}
	// A meeting booked over a session that is already running ends it at once.
	s4 := start(t)
	s4.Settle(t0.Add(time.Minute), []domain.Busy{{Title: "Ad hoc", Start: t0.Add(-5 * time.Minute), End: t0.Add(10 * time.Minute)}})
	if s4.State != domain.Interrupted || !s4.EndedAt.Equal(t0) || s4.Focused(t0.Add(time.Hour)) != 0 {
		t.Fatalf("booked over = %+v", s4)
	}
	// Settling a finished session changes nothing.
	s3.Settle(t0.Add(2*time.Hour), nil)
	if s3.State != domain.Interrupted {
		t.Fatal("finished sessions are immutable")
	}
	if domain.Running.Active() != true || domain.Completed.Active() {
		t.Fatal("active states")
	}
}
