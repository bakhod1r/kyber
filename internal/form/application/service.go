// Package application holds the Forms context's use cases: designing a project's forms and
// submitting them as new issues.
package application

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/form/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("insufficient project role")
)

// Need is the project role an action needs.
type Need int

const (
	NeedBrowse Need = iota + 1 // see the forms
	NeedWrite                  // submit a form (it creates an issue)
	NeedAdmin                  // design forms
)

// Ports to the other contexts (anti-corruption layers live in infrastructure).
type (
	Projects interface {
		Authorize(ctx context.Context, actor, project string, need Need) error
	}
	// Issues creates the issue a submission becomes (ACL to Issue Tracking) and returns its key.
	Issues interface {
		Create(ctx context.Context, actor, project string, s domain.Submission) (string, error)
	}
)

type Deps struct {
	Projects Projects
	Issues   Issues
	Forms    domain.Repository
	NewID    func() string
}

type Service struct{ d Deps }

func NewService(d Deps) *Service { return &Service{d: d} }

// Design is a form as its designer writes it.
type Design struct {
	Name, Intro, IssueType string
	Questions              []domain.QuestionInput
}

func (s *Service) Forms(ctx context.Context, actor, project string) ([]*domain.Form, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, NeedBrowse); err != nil {
		return nil, err
	}
	return s.d.Forms.ForProject(ctx, project)
}

func (s *Service) Create(ctx context.Context, actor, project string, d Design) (*domain.Form, error) {
	if err := s.d.Projects.Authorize(ctx, actor, project, NeedAdmin); err != nil {
		return nil, err
	}
	f, err := domain.NewForm(domain.FormID(s.d.NewID()), project, d.Name, d.Intro, d.IssueType, d.Questions)
	if err != nil {
		return nil, err
	}
	return f, s.d.Forms.Save(ctx, f)
}

// load finds a form and checks the actor's role on its project; forms of hidden projects stay hidden.
func (s *Service) load(ctx context.Context, actor, id string, need Need) (*domain.Form, error) {
	f, err := s.d.Forms.ByID(ctx, domain.FormID(id))
	if err != nil {
		return nil, err
	}
	if err := s.d.Projects.Authorize(ctx, actor, f.Project, need); err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, domain.ErrFormNotFound
		}
		return nil, err
	}
	return f, nil
}

func (s *Service) Form(ctx context.Context, actor, id string) (*domain.Form, error) {
	return s.load(ctx, actor, id, NeedBrowse)
}

// Redesign changes the form loaded at version.
func (s *Service) Redesign(ctx context.Context, actor, id string, version int, d Design) (*domain.Form, error) {
	f, err := s.load(ctx, actor, id, NeedAdmin)
	if err != nil {
		return nil, err
	}
	if f.Version != version {
		return nil, domain.ErrFormConflict
	}
	if err := f.Redesign(d.Name, d.Intro, d.IssueType, d.Questions); err != nil {
		return nil, err
	}
	return f, s.d.Forms.Save(ctx, f)
}

func (s *Service) Delete(ctx context.Context, actor, id string) error {
	f, err := s.load(ctx, actor, id, NeedAdmin)
	if err != nil {
		return err
	}
	return s.d.Forms.Delete(ctx, f.ID)
}

// Submit fills in a form; the answers become a new issue in the form's project, whose key is returned.
func (s *Service) Submit(ctx context.Context, actor, id, summary string, answers map[string]string) (string, error) {
	f, err := s.load(ctx, actor, id, NeedWrite)
	if err != nil {
		return "", err
	}
	sub, err := f.Submit(summary, answers)
	if err != nil {
		return "", err
	}
	return s.d.Issues.Create(ctx, actor, f.Project, sub)
}
