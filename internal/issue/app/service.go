// Package app holds the Issue Tracking context use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidKey      = errors.New("invalid issue key")
	ErrForbidden       = errors.New("insufficient project role")
	ErrInvalidAssignee = errors.New("assignee must be a member of the project")
	ErrInvalidSprint   = errors.New("sprint must be a planned or active sprint of this project")
	ErrInvalidAnchor   = errors.New("give exactly one of after/before: an issue key of the same project")
)

// Sprints is the anti-corruption port to the Agile context.
type Sprints interface {
	// CanHold returns ErrInvalidSprint unless the sprint belongs to project and is not closed.
	CanHold(ctx context.Context, project, sprint string) error
}

// Directory answers membership and naming questions about users (ACL to Project/Identity).
type Directory interface {
	IsMember(ctx context.Context, project, user string) (bool, error)
	DisplayName(ctx context.Context, user string) (string, error)
}

// Access is the anti-corruption port to the Project context's membership rules.
// Non-members get ErrProjectNotFound, members lacking the permission ErrForbidden.
type Access interface {
	Authorize(ctx context.Context, actor, project string, write bool) error
}

type Service struct {
	issues    domain.Repository
	comments  domain.CommentRepository
	keys      domain.KeyAllocator
	access    Access
	directory Directory
	sprints   Sprints
	workflow  *domain.Workflow
	newID     func() string
	now       func() time.Time
}

// Deps are the ports the Issue Tracking use cases depend on.
type Deps struct {
	Issues    domain.Repository
	Comments  domain.CommentRepository
	Keys      domain.KeyAllocator
	Access    Access
	Directory Directory
	Sprints   Sprints
	Workflow  *domain.Workflow
	NewID     func() string
	Now       func() time.Time
}

func NewService(d Deps) *Service {
	return &Service{issues: d.Issues, comments: d.Comments, keys: d.Keys, access: d.Access,
		directory: d.Directory, sprints: d.Sprints, workflow: d.Workflow, newID: d.NewID, now: d.Now}
}

type CreateIssue struct {
	Project string
	Title   string
	Type    string
}

func (s *Service) Create(ctx context.Context, actor string, cmd CreateIssue) (*domain.Issue, error) {
	typ, err := domain.ParseIssueType(cmd.Type)
	if err != nil {
		return nil, err
	}
	// Validate before allocating so rejected commands do not burn issue numbers.
	if _, err := domain.NormalizeTitle(cmd.Title); err != nil {
		return nil, err
	}
	if err := s.access.Authorize(ctx, actor, cmd.Project, true); err != nil {
		return nil, err
	}
	key, err := s.keys.Next(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	is, err := domain.NewIssue(domain.IssueID(s.newID()), key, cmd.Title, typ, s.workflow)
	if err != nil {
		return nil, err
	}
	last, err := s.issues.LastRank(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	is.Rerank(domain.RankBetween(last, "")) // new issues go to the bottom of the backlog
	return is, s.save(ctx, is)
}

func (s *Service) Get(ctx context.Context, actor, rawKey string) (*domain.Issue, error) {
	return s.load(ctx, actor, rawKey, false)
}

// load parses the key and authorizes before touching the issue, so outsiders
// cannot distinguish missing issues from hidden ones.
func (s *Service) load(ctx context.Context, actor, rawKey string, write bool) (*domain.Issue, error) {
	key, err := domain.ParseIssueKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if err := s.access.Authorize(ctx, actor, key.Project(), write); err != nil {
		return nil, err
	}
	return s.issues.ByKey(ctx, key)
}

func (s *Service) Transition(ctx context.Context, actor, rawKey, to string) (*domain.Issue, error) {
	is, err := s.load(ctx, actor, rawKey, true)
	if err != nil {
		return nil, err
	}
	if err := is.Transition(domain.StatusID(to), s.workflow); err != nil {
		return nil, err
	}
	return is, s.save(ctx, is)
}

// Backlog selects issues that are in no sprint.
const Backlog = "backlog"

// ListQuery filters a project listing; empty fields do not filter.
// Sprint is a sprint ID or Backlog.
type ListQuery struct {
	Status string
	Sprint string
}

func (s *Service) List(ctx context.Context, actor, project string, q ListQuery) ([]*domain.Issue, error) {
	if err := s.access.Authorize(ctx, actor, project, false); err != nil {
		return nil, err
	}
	var f domain.ListFilter
	if q.Status != "" {
		st := domain.StatusID(q.Status)
		f.Status = &st
	}
	if q.Sprint != "" {
		sp := domain.SprintID(q.Sprint)
		if q.Sprint == Backlog {
			sp = ""
		}
		f.Sprint = &sp
	}
	return s.issues.ListByProject(ctx, project, f)
}

func (s *Service) save(ctx context.Context, is *domain.Issue) error {
	// The repository writes the aggregate and its events atomically (transactional outbox).
	return s.issues.Save(ctx, is, is.PullEvents())
}
