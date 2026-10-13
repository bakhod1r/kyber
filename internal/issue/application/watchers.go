package application

import (
	"context"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// Watcher is a person watching an issue.
type Watcher struct {
	ID, Name string
}

func (s *Service) autoWatch(ctx context.Context, issue domain.IssueID, user domain.UserID) error {
	if s.watchers == nil || user == "" {
		return nil
	}
	return s.watchers.Watch(ctx, issue, user)
}

// Watch makes the actor watch the issue (anyone who can see it).
func (s *Service) Watch(ctx context.Context, actor, rawKey string) error {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil {
		return err
	}
	return s.autoWatch(ctx, is.ID(), domain.UserID(actor))
}

// Unwatch stops the actor watching the issue.
func (s *Service) Unwatch(ctx context.Context, actor, rawKey string) error {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil || s.watchers == nil {
		return err
	}
	return s.watchers.Unwatch(ctx, is.ID(), domain.UserID(actor))
}

// Watchers lists the issue's watchers by name and whether the actor is one of them.
func (s *Service) Watchers(ctx context.Context, actor, rawKey string) ([]Watcher, bool, error) {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil || s.watchers == nil {
		return nil, false, err
	}
	ids, err := s.watchers.Watchers(ctx, is.ID())
	if err != nil {
		return nil, false, err
	}
	out := make([]Watcher, 0, len(ids))
	watching := false
	for _, id := range ids {
		name, err := s.directory.DisplayName(ctx, string(id))
		if err != nil {
			return nil, false, err
		}
		out = append(out, Watcher{ID: string(id), Name: name})
		watching = watching || string(id) == actor
	}
	return out, watching, nil
}

// WatchersOf lists the issue's watcher ids without authorization, for the Notification context
// (it checks each recipient may still see the issue).
func (s *Service) WatchersOf(ctx context.Context, rawKey string) ([]string, error) {
	is, err := s.Peek(ctx, rawKey)
	if err != nil || s.watchers == nil {
		return nil, err
	}
	ids, err := s.watchers.Watchers(ctx, is.ID())
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out, err
}
