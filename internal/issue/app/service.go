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
	ErrProjectNotFound      = errors.New("project not found")
	ErrInvalidKey           = errors.New("invalid issue key")
	ErrForbidden            = errors.New("insufficient project role")
	ErrInvalidAssignee      = errors.New("assignee must be a project member who can work on issues (not a viewer)")
	ErrInvalidSprint        = errors.New("sprint must be a planned or active sprint of this project")
	ErrInvalidAnchor        = errors.New("give exactly one of after/before: an issue key of the same project")
	ErrStatusInUse          = errors.New("status is still used by issues; move them before removing it")
	ErrInvalidSecurityLevel = errors.New("unknown security level")
	ErrInvalidVersion       = errors.New("fix version must be an unarchived version of this project")
)

// Sprints is the anti-corruption port to the Agile context.
type Sprints interface {
	// CanHold returns ErrInvalidSprint unless the sprint belongs to project and is not closed.
	CanHold(ctx context.Context, project, sprint string) error
	// CanFix returns ErrInvalidVersion unless the version belongs to project and is not archived.
	CanFix(ctx context.Context, project, version string) error
}

// Directory answers membership and naming questions about users (ACL to Project/Identity).
type Directory interface {
	IsMember(ctx context.Context, project, user string) (bool, error)
	// IsAssignable: may the user be assigned issues (Jira's Assignable User; viewers may not).
	IsAssignable(ctx context.Context, project, user string) (bool, error)
	DisplayName(ctx context.Context, user string) (string, error)
}

// Permission is a Jira permission name, decided by the Access context's permission scheme.
type Permission string

const (
	PermBrowse     Permission = "BROWSE_PROJECTS"
	PermCreate     Permission = "CREATE_ISSUES"
	PermEdit       Permission = "EDIT_ISSUES"
	PermAssign     Permission = "ASSIGN_ISSUES"
	PermTransition Permission = "TRANSITION_ISSUES"
	PermClose      Permission = "CLOSE_ISSUES"
	PermComment    Permission = "ADD_COMMENTS"
	PermAdminister Permission = "ADMINISTER_PROJECTS"
)

// Access is the anti-corruption port to the Access context (ADR-0003): may actor exercise perm in
// project, on the issue when given? Hidden projects/issues give ErrProjectNotFound, missing
// permissions ErrForbidden.
type Access interface {
	Can(ctx context.Context, actor, project string, perm Permission, on *domain.Issue) error
}

type Service struct {
	issues    domain.Repository
	comments  domain.CommentRepository
	keys      domain.KeyAllocator
	access    Access
	directory Directory
	sprints   Sprints
	workflow  *domain.Workflow
	workflows domain.WorkflowRepository
	history   domain.HistoryRepository
	links     domain.LinkRepository
	levels    []string
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
	Workflow  *domain.Workflow // the default for projects without a configured workflow
	Workflows domain.WorkflowRepository
	History   domain.HistoryRepository
	Links     domain.LinkRepository
	// SecurityLevels are the issue security levels that can be set (ids, see the Access context).
	SecurityLevels []string
	NewID          func() string
	Now            func() time.Time
}

func NewService(d Deps) *Service {
	return &Service{issues: d.Issues, comments: d.Comments, keys: d.Keys, access: d.Access,
		directory: d.Directory, sprints: d.Sprints, workflow: d.Workflow,
		workflows: d.Workflows, history: d.History, links: d.Links, levels: d.SecurityLevels, newID: d.NewID, now: d.Now}
}

type CreateIssue struct {
	Project string
	Title   string
	Type    string
	Parent  string // issue key of the epic (for stories, tasks, bugs) or of the issue a sub-task belongs to
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
	if err := s.access.Can(ctx, actor, cmd.Project, PermCreate, nil); err != nil {
		return nil, err
	}
	parent, err := s.parentFor(ctx, cmd.Project, cmd.Parent, typ)
	if err != nil {
		return nil, err
	}
	key, err := s.keys.Next(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	wf, err := s.workflowFor(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	is, err := domain.NewIssue(domain.IssueID(s.newID()), key, cmd.Title, typ, domain.UserID(actor), wf)
	if err != nil {
		return nil, err
	}
	if parent != nil {
		if err := is.SetParent(parent); err != nil {
			return nil, err
		}
	}
	last, err := s.issues.LastRank(ctx, cmd.Project)
	if err != nil {
		return nil, err
	}
	is.Rerank(domain.RankBetween(last, "")) // new issues go to the bottom of the backlog
	return is, s.save(ctx, actor, is)
}

func (s *Service) Get(ctx context.Context, actor, rawKey string) (*domain.Issue, error) {
	return s.load(ctx, actor, rawKey, PermBrowse)
}

// load parses the key and authorizes before touching the issue, so outsiders
// cannot distinguish missing issues from hidden ones.
// The issue's own attributes (reporter, assignee, status) then decide perm (ABAC).
func (s *Service) load(ctx context.Context, actor, rawKey string, perm Permission) (*domain.Issue, error) {
	key, err := domain.ParseIssueKey(rawKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	if err := s.access.Can(ctx, actor, key.Project(), PermBrowse, nil); err != nil {
		return nil, err
	}
	is, err := s.issues.ByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := s.access.Can(ctx, actor, key.Project(), perm, is); err != nil {
		return nil, err
	}
	return is, nil
}

// Transition moves the issue; entering a Done status resolves it as resolution ("" = done).
func (s *Service) Transition(ctx context.Context, actor, rawKey, to, resolution string) (*domain.Issue, error) {
	var res domain.Resolution
	if resolution != "" {
		r, err := domain.ParseResolution(resolution)
		if err != nil {
			return nil, err
		}
		res = r
	}
	is, err := s.load(ctx, actor, rawKey, PermTransition)
	if err != nil {
		return nil, err
	}
	wf, err := s.workflowFor(ctx, is.Key().Project())
	if err != nil {
		return nil, err
	}
	// Moving an issue into a Done-category status resolves it: Jira's Close Issues permission.
	if wf.IsDone(domain.StatusID(to)) {
		if err := s.access.Can(ctx, actor, is.Key().Project(), PermClose, is); err != nil {
			return nil, err
		}
	}
	if err := is.TransitionResolved(domain.StatusID(to), wf, res); err != nil {
		return nil, err
	}
	return is, s.save(ctx, actor, is)
}

// Backlog selects issues that are in no sprint.
const Backlog = "backlog"

// ListQuery filters a project listing; empty fields do not filter.
// Sprint is a sprint ID or Backlog.
type ListQuery struct {
	Status     string
	Sprint     string
	FixVersion string
}

func (s *Service) List(ctx context.Context, actor, project string, q ListQuery) ([]*domain.Issue, error) {
	if err := s.access.Can(ctx, actor, project, PermBrowse, nil); err != nil {
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
	if q.FixVersion != "" {
		v := domain.VersionID(q.FixVersion)
		f.FixVersion = &v
	}
	list, err := s.issues.ListByProject(ctx, project, f)
	if err != nil {
		return nil, err
	}
	return s.visible(ctx, actor, project, list)
}

// visible drops the issues the actor may not see (issue security levels, ADR-0003): every list
// is filtered by the same decision as a single read, so a hidden issue never leaks through one.
func (s *Service) visible(ctx context.Context, actor, project string, list []*domain.Issue) ([]*domain.Issue, error) {
	out := make([]*domain.Issue, 0, len(list))
	for _, is := range list {
		if is.SecurityLevel() == "" {
			out = append(out, is)
			continue
		}
		err := s.access.Can(ctx, actor, project, PermBrowse, is)
		if errors.Is(err, ErrProjectNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, is)
	}
	return out, nil
}

// save writes the issue, its events and its changelog (who changed what) atomically
// through the transactional outbox, so every action is kept in the issue's history.
func (s *Service) save(ctx context.Context, actor string, is *domain.Issue) error {
	events := is.PullEvents()
	if ch, ok := is.Changelog(domain.UserID(actor)); ok {
		events = append(events, ch)
	}
	return s.issues.Save(ctx, is, events)
}
