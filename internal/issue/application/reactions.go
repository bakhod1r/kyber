package application

import (
	"context"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// React puts the emoji on the comment for the actor (anyone who can see its issue); idempotent.
func (s *Service) React(ctx context.Context, actor, commentID, emoji string) error {
	r, err := s.reaction(ctx, actor, commentID, emoji)
	if err != nil {
		return err
	}
	return s.reactions.React(ctx, r, s.now())
}

// Unreact takes the actor's emoji off the comment; idempotent.
func (s *Service) Unreact(ctx context.Context, actor, commentID, emoji string) error {
	r, err := s.reaction(ctx, actor, commentID, emoji)
	if err != nil {
		return err
	}
	return s.reactions.Unreact(ctx, r)
}

func (s *Service) reaction(ctx context.Context, actor, commentID, emoji string) (domain.Reaction, error) {
	e, err := domain.ParseEmoji(emoji)
	if err != nil {
		return domain.Reaction{}, err
	}
	if s.reactions == nil {
		return domain.Reaction{}, domain.ErrCommentNotFound
	}
	id := domain.CommentID(commentID)
	key, err := s.comments.IssueKeyOf(ctx, id)
	if err != nil {
		return domain.Reaction{}, err
	}
	if _, err := s.load(ctx, actor, key.String(), PermBrowse); err != nil {
		return domain.Reaction{}, err
	}
	return domain.Reaction{Comment: id, User: domain.UserID(actor), Emoji: e}, nil
}

func (s *Service) tallyReactions(ctx context.Context, actor string, list []CommentView) error {
	if s.reactions == nil || len(list) == 0 {
		return nil
	}
	ids := make([]domain.CommentID, len(list))
	for i, c := range list {
		ids[i] = c.ID()
	}
	rs, err := s.reactions.ForComments(ctx, ids)
	if err != nil {
		return err
	}
	counts := domain.TallyReactions(rs, domain.UserID(actor))
	for i := range list {
		list[i].Reactions = counts[list[i].ID()]
	}
	return nil
}
