package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// WorkflowID identifies a workflow.
type WorkflowID string

// Trigger is what starts a workflow (S80).
type Trigger string

const (
	OnJoin     Trigger = "member_joined" // someone joined the channel: post a welcome
	OnSchedule Trigger = "schedule"      // on the chosen days at a time (UTC): post a message
)

// Days is a set of weekdays: bit 0 = Sunday … bit 6 = Saturday.
type Days uint8

const (
	Weekdays Days = 0b0111110
	Everyday Days = 0b1111111
)

func (d Days) Has(w time.Weekday) bool { return d&(1<<uint(w)) != 0 }

// Workflow is a simple automation in a channel (S80), managed by its owner and admins.
type Workflow struct {
	ID           WorkflowID
	Workspace    WorkspaceID
	Conversation ConversationID
	Trigger      Trigger
	Minute       int  // schedule: minutes after midnight UTC
	Days         Days // schedule: on which days
	Text         string
	CreatedBy    UserID
	CreatedAt    time.Time
	LastRun      time.Time // schedule: the occurrence last posted
}

var (
	ErrInvalidWorkflow  = errors.New("a workflow runs on join, or on chosen days at a time of day, and posts a message")
	ErrWorkflowNotFound = errors.New("workflow not found")
)

func NewWorkflow(id WorkflowID, ws WorkspaceID, conv ConversationID, trig Trigger, minute int, days Days, text string, by UserID, now time.Time) (*Workflow, error) {
	b, err := body(text)
	if err != nil {
		return nil, err
	}
	switch trig {
	case OnJoin:
		minute, days = 0, 0
	case OnSchedule:
		if minute < 0 || minute >= 24*60 || days == 0 || days > Everyday {
			return nil, ErrInvalidWorkflow
		}
	default:
		return nil, ErrInvalidWorkflow
	}
	return &Workflow{ID: id, Workspace: ws, Conversation: conv, Trigger: trig, Minute: minute, Days: days, Text: b, CreatedBy: by,
		CreatedAt: now.UTC(), LastRun: now.UTC()}, nil
}

// Welcome is the join message, with {name} replaced by the newcomer's name.
func (w *Workflow) Welcome(name string) string { return strings.ReplaceAll(w.Text, "{name}", name) }

// Due is the latest scheduled occurrence at or before now that has not run yet (zero when none). Missed ones
// (the server was down) collapse into the latest.
func (w *Workflow) Due(now time.Time) time.Time {
	if w.Trigger != OnSchedule {
		return time.Time{}
	}
	now = now.UTC()
	for back := 0; back < 8; back++ {
		day := time.Date(now.Year(), now.Month(), now.Day()-back, 0, 0, 0, 0, time.UTC)
		at := day.Add(time.Duration(w.Minute) * time.Minute)
		if at.After(now) || !w.Days.Has(day.Weekday()) {
			continue
		}
		if at.After(w.LastRun) {
			return at
		}
		return time.Time{}
	}
	return time.Time{}
}

// Ran records that the occurrence at was posted.
func (w *Workflow) Ran(at time.Time) { w.LastRun = at.UTC() }

// Workflows stores workflows.
type Workflows interface {
	Add(ctx context.Context, w *Workflow) error
	Remove(ctx context.Context, id WorkflowID) error
	ByID(ctx context.Context, id WorkflowID) (*Workflow, error) // ErrWorkflowNotFound
	In(ctx context.Context, conv ConversationID) ([]*Workflow, error)
	// Scheduled lists every workspace's schedule workflows.
	Scheduled(ctx context.Context) ([]*Workflow, error)
	// MarkRan moves LastRun from prev to w.LastRun; false when another replica ran it first.
	MarkRan(ctx context.Context, w *Workflow, prev time.Time) (bool, error)
}
