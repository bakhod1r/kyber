package app

import (
	"context"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// EditIssue edits an issue loaded at Version. Nil fields stay unchanged;
// AssigneeSet with an empty Assignee unassigns.
type EditIssue struct {
	Version     int
	Title       *string
	Description *string
	Priority    *string
	AssigneeSet bool
	Assignee    string
}

func (s *Service) Edit(ctx context.Context, actor, rawKey string, cmd EditIssue) (*domain.Issue, error) {
	is, err := s.load(ctx, actor, rawKey, true)
	if err != nil {
		return nil, err
	}
	if is.Version() != cmd.Version {
		return nil, domain.ErrConcurrentModification
	}
	if cmd.AssigneeSet && cmd.Assignee != "" {
		ok, err := s.directory.IsMember(ctx, is.Key().Project(), cmd.Assignee)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidAssignee
		}
	}
	changes := domain.Changes{Title: cmd.Title, Description: cmd.Description}
	if cmd.Priority != nil {
		p := domain.Priority(*cmd.Priority)
		changes.Priority = &p
	}
	if err := is.Edit(changes); err != nil {
		return nil, err
	}
	if cmd.AssigneeSet {
		is.Assign(domain.UserID(cmd.Assignee))
	}
	return is, s.save(ctx, is)
}

// CommentView is a comment plus its author's display name.
type CommentView struct {
	*domain.Comment
	AuthorName string
}

// AddComment requires write access (viewers read only).
func (s *Service) AddComment(ctx context.Context, actor, rawKey, body string) (*domain.Comment, error) {
	is, err := s.load(ctx, actor, rawKey, true)
	if err != nil {
		return nil, err
	}
	c, err := domain.NewComment(domain.CommentID(s.newID()), is.ID(), is.Key(), domain.UserID(actor), body, s.now())
	if err != nil {
		return nil, err
	}
	return c, s.comments.Add(ctx, c, c.PullEvents())
}

func (s *Service) Comments(ctx context.Context, actor, rawKey string) ([]CommentView, error) {
	is, err := s.load(ctx, actor, rawKey, false)
	if err != nil {
		return nil, err
	}
	list, err := s.comments.ListByIssue(ctx, is.ID())
	if err != nil {
		return nil, err
	}
	out := make([]CommentView, 0, len(list))
	names := map[domain.UserID]string{}
	for _, c := range list {
		name, ok := names[c.Author()]
		if !ok {
			if name, err = s.directory.DisplayName(ctx, string(c.Author())); err != nil {
				return nil, err
			}
			names[c.Author()] = name
		}
		out = append(out, CommentView{Comment: c, AuthorName: name})
	}
	return out, nil
}

// DisplayName exposes the directory's name lookup to adapters.
func (s *Service) DisplayName(ctx context.Context, user string) (string, error) {
	return s.directory.DisplayName(ctx, user)
}
