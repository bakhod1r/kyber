// Package app holds the Focus context use cases: Pomodoro sessions on issues and the work log.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/focus/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrCannotWork    = errors.New("your project role cannot work on issues (viewers can only read)")
)

// Ports to other contexts (anti-corruption layers live in adapter/acl).
type (
	// Calendar lists the user's meetings overlapping [from, to) in every workspace.
	Calendar interface {
		Busy(ctx context.Context, user string, from, to time.Time) ([]domain.Busy, error)
	}
	// Issues returns ErrIssueNotFound unless user may view the issue, and ErrCannotWork
	// when they may view but not work on it (Jira's Work On Issues).
	Issues interface {
		CanView(ctx context.Context, user, key string) error
		CanWorkOn(ctx context.Context, user, key string) error
	}
	Clock interface{ Now() time.Time }
)

type Deps struct {
	Sessions domain.Repository
	Calendar Calendar
	Issues   Issues
	NewID    func() string
	Clock    Clock
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

func workspace(ctx context.Context) string {
	if ws, ok := tenant.From(ctx); ok {
		return ws
	}
	return tenant.Default
}

// current loads the user's active session brought up to date (it may have completed or been
// interrupted by a meeting since); ErrSessionNotFound when there is none any more.
func (s *Service) current(ctx context.Context, user string) (*domain.Session, error) {
	sess, err := s.d.Sessions.Active(ctx, user)
	if err != nil {
		return nil, err
	}
	now := s.d.Clock.Now()
	busy, err := s.d.Calendar.Busy(ctx, user, sess.StartedAt, now)
	if err != nil {
		return nil, err
	}
	sess.Settle(now, busy)
	if err := s.d.Sessions.Save(ctx, sess); err != nil {
		return nil, err
	}
	if !sess.State.Active() {
		return nil, domain.ErrSessionNotFound
	}
	return sess, nil
}

func (s *Service) Current(ctx context.Context, user string) (*domain.Session, error) {
	return s.current(ctx, user)
}

// Start begins a Pomodoro on an issue the user can see, unless a meeting is in the way.
func (s *Service) Start(ctx context.Context, user, issue string, minutes int) (*domain.Session, error) {
	if err := s.d.Issues.CanWorkOn(ctx, user, issue); err != nil {
		return nil, err
	}
	if _, err := s.current(ctx, user); err == nil {
		return nil, domain.ErrAlreadyRunning
	} else if !errors.Is(err, domain.ErrSessionNotFound) {
		return nil, err
	}
	planned := time.Duration(minutes) * time.Minute
	now := s.d.Clock.Now()
	busy, err := s.d.Calendar.Busy(ctx, user, now, now.Add(planned))
	if err != nil {
		return nil, err
	}
	sess, err := domain.Start(s.d.NewID(), user, workspace(ctx), issue, now, planned, busy)
	if err != nil {
		return nil, err
	}
	return sess, s.d.Sessions.Create(ctx, sess)
}

func (s *Service) change(ctx context.Context, user string, fn func(*domain.Session, time.Time) error) (*domain.Session, error) {
	sess, err := s.current(ctx, user)
	if err != nil {
		return nil, err
	}
	if err := fn(sess, s.d.Clock.Now()); err != nil {
		return nil, err
	}
	return sess, s.d.Sessions.Save(ctx, sess)
}

func (s *Service) Pause(ctx context.Context, user string) (*domain.Session, error) {
	return s.change(ctx, user, func(x *domain.Session, now time.Time) error { return x.Pause(now) })
}

// Resume re-checks the calendar for the remaining focus time.
func (s *Service) Resume(ctx context.Context, user string) (*domain.Session, error) {
	return s.change(ctx, user, func(x *domain.Session, now time.Time) error {
		busy, err := s.d.Calendar.Busy(ctx, user, now, now.Add(x.Planned))
		if err != nil {
			return err
		}
		return x.Resume(now, busy)
	})
}

func (s *Service) Stop(ctx context.Context, user string) (*domain.Session, error) {
	return s.change(ctx, user, func(x *domain.Session, now time.Time) error { return x.Stop(now) })
}

// IssueLog is the issue's Pomodoro history in this workspace and the total focus time.
func (s *Service) IssueLog(ctx context.Context, user, issue string) ([]*domain.Session, time.Duration, error) {
	if err := s.d.Issues.CanView(ctx, user, issue); err != nil {
		return nil, 0, err
	}
	_, _ = s.current(ctx, user) // settle the viewer's own running session first
	log, err := s.d.Sessions.ForIssue(ctx, workspace(ctx), issue)
	if err != nil {
		return nil, 0, err
	}
	var total time.Duration
	now := s.d.Clock.Now()
	for _, x := range log {
		total += x.Focused(now)
	}
	return log, total, nil
}

// MyLog is the user's own sessions started in [from, to).
func (s *Service) MyLog(ctx context.Context, user string, from, to time.Time) ([]*domain.Session, error) {
	_, _ = s.current(ctx, user)
	return s.d.Sessions.ForUser(ctx, user, from, to)
}
