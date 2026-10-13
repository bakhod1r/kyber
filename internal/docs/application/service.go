// Package application holds the Docs context's use cases: a project's pages and their history.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/docs/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("insufficient project role")
)

// Projects checks the actor's role on a project: reading needs browse, writing needs write
// (ACL to Project).
type Projects interface {
	Authorize(ctx context.Context, actor, project string, change bool) error
}

type Deps struct {
	Projects Projects
	Pages    domain.Repository
	NewID    func() string
	Now      func() time.Time
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Pages lists the project's pages by title.
func (s *Service) Pages(ctx context.Context, actor, project string) ([]*domain.Page, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	return s.d.Pages.ForProject(ctx, project)
}

// Create writes a new page.
func (s *Service) Create(ctx context.Context, actor, project, title, body string) (*domain.Page, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, true); err != nil {
		return nil, err
	}
	p, rev, err := domain.NewPage(domain.PageID(s.d.NewID()), project, title, body, actor, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	return p, s.d.Pages.Save(ctx, p, rev)
}

// load finds a page and checks the actor's role on its project; pages of hidden projects stay hidden.
func (s *Service) load(ctx context.Context, actor, id string, change bool) (*domain.Page, error) {
	p, err := s.d.Pages.ByID(ctx, domain.PageID(id))
	if err != nil {
		return nil, err
	}
	if err := s.d.Projects.Authorize(ctx, actor, p.Project, change); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrPageNotFound
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) Page(ctx context.Context, actor, id string) (*domain.Page, error) {
	return s.load(ctx, actor, id, false)
}

// Revise saves a new title and body over the page loaded at version.
func (s *Service) Revise(ctx context.Context, actor, id string, version int, title, body string) (*domain.Page, error) {
	p, err := s.load(ctx, actor, id, true)
	if err != nil {
		return nil, err
	}
	if p.Version != version {
		return nil, domain.ErrPageConflict
	}
	rev, changed, err := p.Revise(title, body, actor, s.d.Now().UTC())
	if err != nil || !changed {
		return p, err
	}
	return p, s.d.Pages.Save(ctx, p, rev)
}

// History lists the page's revisions, newest first.
func (s *Service) History(ctx context.Context, actor, id string) ([]domain.Revision, error) {
	p, err := s.load(ctx, actor, id, false)
	if err != nil {
		return nil, err
	}
	return s.d.Pages.History(ctx, p.ID)
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	p, err := s.load(ctx, actor, id, true)
	if err != nil {
		return err
	}
	return s.d.Pages.Delete(ctx, p.ID)
}
