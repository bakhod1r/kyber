// Package domain is the Focus context: Pomodoro sessions on issues and their history (the
// work log). Meetings on the calendar block focus: a session cannot start, resume or run
// through a meeting of its user.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultFocus = 25 * time.Minute
	MinFocus     = 15 * time.Minute
	MaxFocus     = 60 * time.Minute
)

var (
	ErrInvalidLength   = errors.New("a focus session lasts 15 to 60 minutes")
	ErrNoIssue         = errors.New("a focus session needs an issue")
	ErrMeetingConflict = errors.New("you have a meeting at that time")
	ErrNotRunning      = errors.New("the session is not running")
	ErrNotPaused       = errors.New("the session is not paused")
	ErrFinished        = errors.New("the session has already ended")
	ErrAlreadyRunning  = errors.New("you already have a focus session")
	ErrSessionNotFound = errors.New("focus session not found")
)

type State string

const (
	Running     State = "running"
	Paused      State = "paused"
	Completed   State = "completed"
	Interrupted State = "interrupted"
)

// Active states belong to the user's one current session.
func (s State) Active() bool { return s == Running || s == Paused }

// Busy is a meeting interval of the user (from the Calendar context).
type Busy struct {
	Title      string
	Start, End time.Time
}

// MeetingConflictError names the meeting that blocks focusing.
type MeetingConflictError struct{ Busy }

func (e *MeetingConflictError) Error() string {
	return fmt.Sprintf("%s: %q %s–%s", ErrMeetingConflict, e.Title, e.Start.Format("15:04"), e.End.Format("15:04"))
}
func (e *MeetingConflictError) Unwrap() error { return ErrMeetingConflict }

func conflict(from, to time.Time, busy []Busy) error {
	for _, b := range busy {
		if from.Before(b.End) && b.Start.Before(to) {
			return &MeetingConflictError{b}
		}
	}
	return nil
}

// Session is one Pomodoro on an issue; finished sessions are the work log.
type Session struct {
	ID, User, Workspace, Issue string
	Planned                    time.Duration
	StartedAt                  time.Time
	State                      State
	PausedAt                   time.Time // zero unless paused
	PausedFor                  time.Duration
	EndedAt                    time.Time // zero while active
	Reason                     string    // why it was interrupted
}

// Start begins a session unless a meeting falls inside it.
func Start(id, user, workspace, issue string, now time.Time, planned time.Duration, busy []Busy) (*Session, error) {
	if planned < MinFocus || planned > MaxFocus {
		return nil, ErrInvalidLength
	}
	if issue = strings.TrimSpace(issue); issue == "" {
		return nil, ErrNoIssue
	}
	if err := conflict(now, now.Add(planned), busy); err != nil {
		return nil, err
	}
	return &Session{ID: id, User: user, Workspace: workspace, Issue: issue, Planned: planned, StartedAt: now.UTC(), State: Running}, nil
}

// Focused is the focus time so far (pauses excluded), at most Planned.
func (s *Session) Focused(now time.Time) time.Duration {
	end := now
	switch {
	case !s.EndedAt.IsZero():
		end = s.EndedAt
	case s.State == Paused:
		end = s.PausedAt
	}
	return min(end.Sub(s.StartedAt)-s.PausedFor, s.Planned)
}

func (s *Session) Pause(now time.Time) error {
	if s.State != Running {
		return ErrNotRunning
	}
	s.State, s.PausedAt = Paused, now
	return nil
}

// Resume continues a paused session if the remaining time is free of meetings.
func (s *Session) Resume(now time.Time, busy []Busy) error {
	if s.State != Paused {
		return ErrNotPaused
	}
	if err := conflict(now, now.Add(s.Planned-s.Focused(now)), busy); err != nil {
		return err
	}
	s.State, s.PausedFor, s.PausedAt = Running, s.PausedFor+now.Sub(s.PausedAt), time.Time{}
	return nil
}

func (s *Session) end(at time.Time, state State, reason string) {
	if s.State == Paused {
		s.PausedFor += at.Sub(s.PausedAt)
		s.PausedAt = time.Time{}
	}
	s.State, s.EndedAt, s.Reason = state, at, reason
}

// Stop ends the session early.
func (s *Session) Stop(now time.Time) error {
	if !s.State.Active() {
		return ErrFinished
	}
	s.end(now, Interrupted, "stopped")
	return nil
}

// Settle brings the session up to date at now: it completes when the planned focus time is
// reached, or is interrupted by a meeting that began meanwhile — whichever came first.
func (s *Session) Settle(now time.Time, busy []Busy) {
	if !s.State.Active() {
		return
	}
	var doneAt time.Time
	if s.State == Running {
		doneAt = s.StartedAt.Add(s.PausedFor + s.Planned)
	}
	for _, b := range busy {
		if b.End.After(s.StartedAt) && !b.Start.After(now) && (doneAt.IsZero() || b.Start.Before(doneAt)) {
			at := b.Start
			if at.Before(s.StartedAt) { // a meeting booked over a session already running
				at = s.StartedAt
			}
			s.end(at, Interrupted, "meeting: "+b.Title)
			return
		}
	}
	if !doneAt.IsZero() && !doneAt.After(now) {
		s.end(doneAt, Completed, "")
	}
}

type Repository interface {
	Create(ctx context.Context, s *Session) error // ErrAlreadyRunning if the user has an active session
	Save(ctx context.Context, s *Session) error
	Active(ctx context.Context, user string) (*Session, error) // ErrSessionNotFound
	// ForIssue and ForUser list finished and active sessions, newest first.
	ForIssue(ctx context.Context, workspace, issue string) ([]*Session, error)
	ForUser(ctx context.Context, user string, from, to time.Time) ([]*Session, error)
}
