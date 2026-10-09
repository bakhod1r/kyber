// Package app holds the Importer context use cases: bringing issues in from other trackers.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/bakhod1r/kyber/internal/importer/domain"
)

// MaxCSVBytes bounds an upload; Jira exports of a few thousand issues fit comfortably.
const MaxCSVBytes = 10 << 20

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrForbidden       = errors.New("only project admins can import")
	ErrTooLarge        = fmt.Errorf("CSV is larger than %d MB", MaxCSVBytes>>20)
	ErrInvalidCSV      = errors.New("not a Jira CSV export")
)

// Access is the anti-corruption port to the Project context.
// Non-members get ErrProjectNotFound, non-admins ErrForbidden.
type Access interface {
	AuthorizeAdmin(ctx context.Context, actor, project string) error
	AuthorizeRead(ctx context.Context, actor, project string) error
}

// Source lists a project's issues for export (ACL to Issue Tracking and Identity).
type Source interface {
	ExportRows(ctx context.Context, project string) ([]domain.ExportRow, error)
}

// Members lists the project's members for matching assignees and reporters.
type Members interface {
	Members(ctx context.Context, actor, project string) ([]domain.Member, error)
}

// Issues is the anti-corruption port to the Issue Tracking context; it returns the new issue key.
type Issues interface {
	Import(ctx context.Context, project, reporter string, it domain.Item) (string, error)
}

type Deps struct {
	Access   Access
	Members  Members
	Issues   Issues
	Mappings domain.MappingRepository
	Source   Source
}

type Service struct {
	d     Deps
	mu    sync.Mutex
	locks map[string]*sync.Mutex // one import at a time per project
}

func NewService(d Deps) *Service { return &Service{d: d, locks: map[string]*sync.Mutex{}} }

// ReportItem is an importable row and, after a real run, the Kyber issue created from it.
type ReportItem struct {
	domain.Item
	IssueKey string
}

type Report struct {
	DryRun  bool
	Items   []ReportItem
	Errors  []domain.RowError
	Skipped []string // external keys imported by an earlier run
}

func (s *Service) lock(project string) func() {
	s.mu.Lock()
	l, ok := s.locks[project]
	if !ok {
		l = &sync.Mutex{}
		s.locks[project] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// Import previews (dryRun) or runs a Jira CSV import into a project. A run stops at the
// first failure and returns what it created so far; re-running resumes where it stopped.
func (s *Service) Import(ctx context.Context, actor, project, csv string, dryRun bool) (Report, error) {
	if len(csv) > MaxCSVBytes {
		return Report{}, ErrTooLarge
	}
	if err := s.d.Access.AuthorizeAdmin(ctx, actor, project); err != nil {
		return Report{}, err
	}
	rows, err := domain.ParseJiraCSV(strings.NewReader(csv))
	if err != nil {
		return Report{}, fmt.Errorf("%w: %w", ErrInvalidCSV, err)
	}
	members, err := s.d.Members.Members(ctx, actor, project)
	if err != nil {
		return Report{}, err
	}
	if !dryRun {
		defer s.lock(project)()
	}
	done, err := s.d.Mappings.ForProject(ctx, project)
	if err != nil {
		return Report{}, err
	}
	imported := make(map[string]bool, len(done))
	for k := range done {
		imported[k] = true
	}
	plan := domain.Plan(rows, members, imported)
	r := Report{DryRun: dryRun, Errors: plan.Errors, Skipped: plan.Skipped}
	for _, it := range plan.Items {
		if dryRun {
			r.Items = append(r.Items, ReportItem{Item: it})
			continue
		}
		reporter := it.ReporterID
		if reporter == "" {
			reporter = actor
		}
		key, err := s.d.Issues.Import(ctx, project, reporter, it)
		if err != nil {
			return r, fmt.Errorf("import %s (line %d): %w", it.Key, it.Line, err)
		}
		if err := s.d.Mappings.Add(ctx, domain.Mapping{Project: project, ExternalKey: it.Key, IssueKey: key}); err != nil {
			return r, fmt.Errorf("record %s → %s: %w", it.Key, key, err)
		}
		r.Items = append(r.Items, ReportItem{Item: it, IssueKey: key})
	}
	return r, nil
}

// Export writes every issue of the project as a Jira-compatible CSV; any member may export.
func (s *Service) Export(ctx context.Context, actor, project string, w io.Writer) error {
	if err := s.d.Access.AuthorizeRead(ctx, actor, project); err != nil {
		return err
	}
	rows, err := s.d.Source.ExportRows(ctx, project)
	if err != nil {
		return err
	}
	return domain.WriteJiraCSV(w, rows)
}
