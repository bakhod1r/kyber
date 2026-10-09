// Package app holds the Calendar context use cases: the workspace time table.
package app

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

var (
	ErrNotMember = errors.New("every attendee must be a member of the workspace")
	ErrForbidden = errors.New("only the organizer can cancel a meeting")
	ErrRange     = errors.New("the time table range must be between 1 minute and 62 days")
)

// Members is the anti-corruption port to the Workspace context.
type Members interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

type Service struct {
	repo    domain.Repository
	members Members
	newID   func() string
	now     func() time.Time
}

func NewService(repo domain.Repository, members Members, newID func() string, now func() time.Time) *Service {
	return &Service{repo: repo, members: members, newID: newID, now: now}
}

func workspace(ctx context.Context) string {
	if ws, ok := tenant.From(ctx); ok {
		return ws
	}
	return tenant.Default
}

type Schedule struct {
	Title      string
	Start, End time.Time
	Attendees  []string
}

// Schedule books a meeting in the request's workspace; the organizer always attends.
func (s *Service) Schedule(ctx context.Context, actor string, in Schedule) (*domain.Meeting, error) {
	ws := workspace(ctx)
	attendees := make([]domain.UserID, 0, len(in.Attendees))
	for _, a := range in.Attendees {
		ok, err := s.members.IsMember(ctx, ws, a)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotMember
		}
		attendees = append(attendees, domain.UserID(a))
	}
	m, err := domain.NewMeeting(domain.MeetingID(s.newID()), domain.WorkspaceID(ws), in.Title, in.Start, in.End, domain.UserID(actor), attendees)
	if err != nil {
		return nil, err
	}
	return m, s.repo.Create(ctx, m)
}

func (s *Service) Cancel(ctx context.Context, actor, id string) error {
	m, err := s.repo.ByID(ctx, domain.MeetingID(id))
	if err != nil {
		return err
	}
	if string(m.Workspace) != workspace(ctx) {
		return domain.ErrMeetingNotFound
	}
	if string(m.Organizer) != actor {
		if !slices.Contains(m.Attendees, domain.UserID(actor)) {
			return domain.ErrMeetingNotFound // outsiders cannot learn that it exists
		}
		return ErrForbidden
	}
	return s.repo.Delete(ctx, m.ID)
}

// Timetable lists the actor's meetings in the request's workspace.
func (s *Service) Timetable(ctx context.Context, actor string, from, to time.Time) ([]*domain.Meeting, error) {
	if d := to.Sub(from); d < time.Minute || d > 62*24*time.Hour {
		return nil, ErrRange
	}
	all, err := s.repo.ForUser(ctx, domain.UserID(actor), from, to)
	if err != nil {
		return nil, err
	}
	ws := domain.WorkspaceID(workspace(ctx))
	out := all[:0]
	for _, m := range all {
		if m.Workspace == ws {
			out = append(out, m)
		}
	}
	return out, nil
}

// BusyFor lists a user's meetings in every workspace (a meeting anywhere blocks focus).
func (s *Service) BusyFor(ctx context.Context, user string, from, to time.Time) ([]*domain.Meeting, error) {
	return s.repo.ForUser(ctx, domain.UserID(user), from, to)
}
