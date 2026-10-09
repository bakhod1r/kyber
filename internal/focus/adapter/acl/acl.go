// Package acl is the Focus context's anti-corruption layer to Calendar and Issue Tracking.
package acl

import (
	"context"
	"errors"
	"time"

	calendardomain "github.com/bakhod1r/kyber/internal/calendar/domain"
	"github.com/bakhod1r/kyber/internal/focus/app"
	"github.com/bakhod1r/kyber/internal/focus/domain"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
	issuedomain "github.com/bakhod1r/kyber/internal/issue/domain"
)

type MeetingSource interface {
	BusyFor(ctx context.Context, user string, from, to time.Time) ([]*calendardomain.Meeting, error)
}

type Calendar struct{ src MeetingSource }

func NewCalendar(src MeetingSource) Calendar { return Calendar{src: src} }

func (c Calendar) Busy(ctx context.Context, user string, from, to time.Time) ([]domain.Busy, error) {
	ms, err := c.src.BusyFor(ctx, user, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Busy, 0, len(ms))
	for _, m := range ms {
		out = append(out, domain.Busy{Title: m.Title, Start: m.Start, End: m.End})
	}
	return out, nil
}

type IssueReader interface {
	Get(ctx context.Context, actor, rawKey string) (*issuedomain.Issue, error)
}

type Issues struct{ src IssueReader }

func NewIssues(src IssueReader) Issues { return Issues{src: src} }

// CanView hides missing, malformed and inaccessible issues alike.
func (i Issues) CanView(ctx context.Context, user, key string) error {
	_, err := i.src.Get(ctx, user, key)
	switch {
	case errors.Is(err, issueapp.ErrProjectNotFound), errors.Is(err, issueapp.ErrInvalidKey), errors.Is(err, issuedomain.ErrIssueNotFound):
		return app.ErrIssueNotFound
	}
	return err
}
