package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// LinkView is a link read from one issue's side: "blocks KYB-2 (Title, status)".
type LinkView struct {
	domain.Link
	Phrase string
	Other  *domain.Issue
}

// Link connects two issues; the actor needs write access to the source issue and read access to the target.
func (s *Service) Link(ctx context.Context, actor, rawFrom, typ, rawTo string) (domain.Link, error) {
	from, err := s.load(ctx, actor, rawFrom, PermEdit)
	if err != nil {
		return domain.Link{}, err
	}
	to, err := s.load(ctx, actor, rawTo, PermBrowse)
	if err != nil {
		return domain.Link{}, err
	}
	l, err := domain.NewLink(domain.LinkID(s.newID()), domain.LinkType(typ), from, to, domain.UserID(actor))
	if err != nil {
		return domain.Link{}, err
	}
	return l, s.links.Add(ctx, l, []domain.Event{l.Linked()})
}

// Unlink removes a link; the actor needs write access to its source issue's project.
func (s *Service) Unlink(ctx context.Context, actor, id string) error {
	l, err := s.links.ByID(ctx, domain.LinkID(id))
	if err != nil {
		return err
	}
	if err := s.access.Can(ctx, actor, l.FromKey.Project(), PermEdit, nil); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return domain.ErrLinkNotFound
		}
		return err
	}
	return s.links.Remove(ctx, l.ID, []domain.Event{l.Unlinked(domain.UserID(actor))})
}

// Links lists an issue's links; links to issues the actor may not read are left out.
func (s *Service) Links(ctx context.Context, actor, rawKey string) ([]LinkView, error) {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil {
		return nil, err
	}
	links, err := s.links.ForIssue(ctx, is.ID())
	if err != nil {
		return nil, err
	}
	out := []LinkView{}
	for _, l := range links {
		other, err := s.load(ctx, actor, l.OtherKey(is.ID()).String(), PermBrowse)
		if errors.Is(err, ErrProjectNotFound) || errors.Is(err, domain.ErrIssueNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("link %s: %w", l.ID, err)
		}
		out = append(out, LinkView{Link: l, Phrase: l.PhraseFrom(is.ID()), Other: other})
	}
	return out, nil
}
