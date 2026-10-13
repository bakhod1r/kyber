// Package app holds the Calendar context use cases: the workspace time table.
package application

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
	ErrInPast    = errors.New("a meeting cannot start in the past")
)

// Members is the anti-corruption port to the Workspace context.
type Members interface {
	IsMember(ctx context.Context, workspace, user string) (bool, error)
}

// Teams are the ports for team events (ACLs to Project and Workspace).
type (
	// Organizers checks who may invite a whole audience: project members with write access for
	// a project, workspace administrators for the workspace (ErrForbidden / ErrProjectNotFound).
	Organizers interface {
		InviteProject(ctx context.Context, actor, project string) error
		InviteWorkspace(ctx context.Context, actor string) error
	}
	// Audience lists the members invited by scope.
	Audience interface {
		ProjectMembers(ctx context.Context, project string) ([]string, error)
		WorkspaceMembers(ctx context.Context) ([]string, error)
	}
)

var ErrProjectNotFound = errors.New("project not found")

type Service struct {
	repo          domain.Repository
	members       Members
	organizers    Organizers
	audience      Audience
	conversations Conversations
	newID         func() string
	now           func() time.Time
}

// WithTeams enables meetings for a whole project or workspace.
func (s *Service) WithTeams(o Organizers, a Audience) *Service {
	s.organizers, s.audience = o, a
	return s
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
	// Scope "project" (with Project) or "workspace" invites every member; "" or "people" invites Attendees.
	Scope   string
	Project string
	// Conversation (optional): the channel whose huddle the meeting happens in (S76); the organizer must read it.
	Conversation string
}

// Conversations tells whether someone may read a chat conversation (Chat context).
type Conversations interface {
	Readable(ctx context.Context, user, conversation string) (bool, error)
}

// ErrNoConversation: the meeting's channel does not exist or the organizer may not read it.
var ErrNoConversation = errors.New("no such channel")

// WithConversations lets meetings happen in a channel's huddle (S76).
func (s *Service) WithConversations(c Conversations) *Service { s.conversations = c; return s }

// Schedule books a meeting in the request's workspace; the organizer always attends. A team
// event invites a project's or the workspace's members (and tells them all).
func (s *Service) Schedule(ctx context.Context, actor string, in Schedule) (*domain.Meeting, error) {
	if in.Start.Before(s.now().Add(-time.Minute)) {
		return nil, ErrInPast
	}
	ws := workspace(ctx)
	aud := domain.Audience{Scope: domain.Scope(in.Scope), Project: in.Project}
	if aud.Scope != domain.ScopeProject {
		aud.Project = "" // only a project event names a project
	}
	switch aud.Scope {
	case domain.ScopeProject:
		if err := s.organizers.InviteProject(ctx, actor, in.Project); err != nil {
			return nil, err
		}
		ids, err := s.audience.ProjectMembers(ctx, in.Project)
		if err != nil {
			return nil, err
		}
		in.Attendees = append(in.Attendees, ids...)
	case domain.ScopeWorkspace:
		if err := s.organizers.InviteWorkspace(ctx, actor); err != nil {
			return nil, err
		}
		ids, err := s.audience.WorkspaceMembers(ctx)
		if err != nil {
			return nil, err
		}
		in.Attendees = append(in.Attendees, ids...)
	}
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
	if in.Conversation != "" {
		ok := false
		if s.conversations != nil {
			var err error
			if ok, err = s.conversations.Readable(ctx, actor, in.Conversation); err != nil {
				return nil, err
			}
		}
		if !ok {
			return nil, ErrNoConversation
		}
	}
	m, err := domain.PlanMeeting(domain.MeetingID(s.newID()), domain.WorkspaceID(ws), in.Title, in.Start, in.End, domain.UserID(actor), attendees, aud)
	if err != nil {
		return nil, err
	}
	m.Conversation = in.Conversation
	return m, s.repo.Create(ctx, m)
}

// visible loads a meeting of the request's workspace.
func (s *Service) visible(ctx context.Context, id string) (*domain.Meeting, error) {
	m, err := s.repo.ByID(ctx, domain.MeetingID(id))
	if err == nil && string(m.Workspace) != workspace(ctx) {
		err = domain.ErrMeetingNotFound
	}
	return m, err
}

// Respond records the actor's answer to an invitation.
func (s *Service) Respond(ctx context.Context, actor, id, answer string) (*domain.Meeting, error) {
	m, err := s.visible(ctx, id)
	if err != nil {
		return nil, err
	}
	r, err := domain.ParseResponse(answer)
	if err != nil {
		return nil, err
	}
	if err := m.Respond(domain.UserID(actor), r); err != nil {
		return nil, domain.ErrMeetingNotFound // outsiders cannot learn that it exists
	}
	return m, s.repo.Respond(ctx, m, domain.UserID(actor))
}

// RemindDue reminds the attendees of every meeting (in any workspace) starting within
// domain.ReminderLead of now, once; it returns how many reminders it sent.
func (s *Service) RemindDue(ctx context.Context) (int, error) {
	now := s.now()
	due, err := s.repo.DueForReminder(ctx, now, now.Add(domain.ReminderLead))
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, m := range due {
		if !m.Remind(now) {
			continue
		}
		err := s.repo.SaveReminder(tenant.With(ctx, string(m.Workspace)), m)
		switch {
		case errors.Is(err, domain.ErrAlreadyReminded): // another replica was first
		case err != nil:
			return sent, err
		default:
			sent++
		}
	}
	return sent, nil
}

func (s *Service) Cancel(ctx context.Context, actor, id string) error {
	m, err := s.visible(ctx, id)
	if err != nil {
		return err
	}
	if string(m.Organizer) != actor {
		if !slices.Contains(m.Attendees, domain.UserID(actor)) {
			return domain.ErrMeetingNotFound // outsiders cannot learn that it exists
		}
		return ErrForbidden
	}
	m.Cancel(domain.UserID(actor))
	return s.repo.Cancel(ctx, m)
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
