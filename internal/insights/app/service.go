// Package app holds the Insights use cases: ingesting activity and building reports.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"time"

	"github.com/bakhod1r/kyber/internal/insights/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrSprintNotFound  = errors.New("sprint not found")
)

// IssueRow is the current state of an issue, for distribution reports.
type IssueRow struct {
	Key, Type, Status, Priority string
	Assignee                    string
	Points                      *int // tenths
}

type SprintInfo struct {
	ID, Project, Name, State string
	Start, End, Completed    time.Time
}

// Ports to other contexts.
type (
	Access interface {
		Authorize(ctx context.Context, actor, project string) error // read access
	}
	Issues interface {
		Issues(ctx context.Context, project string) ([]IssueRow, error)
	}
	Users interface {
		DisplayName(ctx context.Context, id string) (string, error)
	}
	Sprints interface {
		Sprint(ctx context.Context, id string) (SprintInfo, error)
		ClosedSprints(ctx context.Context, project string, limit int) ([]SprintInfo, error)
	}
)

type Deps struct {
	Repo    domain.Repository
	Access  Access
	Issues  Issues
	Users   Users
	Sprints Sprints
	Now     func() time.Time
	Log     *slog.Logger
}

type Service struct{ d Deps }

func NewService(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

var keySuffix = regexp.MustCompile(`-[0-9]+$`)

// OnIssueEvent records issue.created / transitioned / sprint_changed / estimated.
func (s *Service) OnIssueEvent(ctx context.Context, m outbox.Message) error {
	var e struct {
		ID, Key  string
		From, To json.RawMessage
	}
	if err := json.Unmarshal(m.Payload, &e); err != nil || e.ID == "" || e.Key == "" {
		s.d.Log.ErrorContext(ctx, "insights: skipping malformed event", "id", m.ID, "name", m.Name)
		return nil
	}
	entry := domain.Entry{Event: m.ID, At: m.At, Project: keySuffix.ReplaceAllString(e.Key, ""), Issue: e.ID}
	str := func(raw json.RawMessage) string {
		var v string
		_ = json.Unmarshal(raw, &v)
		return v
	}
	pts := func(raw json.RawMessage) *int {
		var f *float64
		if json.Unmarshal(raw, &f) != nil || f == nil {
			return nil
		}
		t := int(math.Round(*f * 10))
		return &t
	}
	switch m.Name {
	case "issue.created":
		entry.Kind, entry.To = domain.KindCreated, "todo"
	case "issue.transitioned":
		entry.Kind, entry.From, entry.To = domain.KindStatus, str(e.From), str(e.To)
	case "issue.sprint_changed":
		entry.Kind, entry.From, entry.To = domain.KindSprint, str(e.From), str(e.To)
	case "issue.estimated":
		entry.Kind, entry.FromPoints, entry.ToPoints = domain.KindEstimate, pts(e.From), pts(e.To)
	default:
		return nil
	}
	return s.d.Repo.Record(ctx, entry)
}

// Count is one slice of a distribution.
type Count struct {
	Key    string `json:"key"`
	Count  int    `json:"count"`
	Points int    `json:"-"` // tenths
}

type Load struct {
	UserID, Name string
	Count        int
	Points       int // tenths
}

type Summary struct {
	Total, Open, Done, Unassigned int
	TotalPoints, OpenPoints       int // tenths
	ByStatus, ByType, ByPriority  []Count
	Workload                      []Load // open issues per assignee
	Cycle                         domain.Cycle
}

var (
	statusOrder   = []string{"todo", "in_progress", "done"}
	typeOrder     = []string{"epic", "story", "task", "bug", "subtask"}
	priorityOrder = []string{"highest", "high", "medium", "low", "lowest"}
)

func ordered(order []string, counts map[string]*Count) []Count {
	out := []Count{}
	for _, k := range order {
		if c, ok := counts[k]; ok {
			out = append(out, *c)
		}
	}
	return out
}

func (s *Service) Summary(ctx context.Context, actor, project string) (Summary, error) {
	if err := s.d.Access.Authorize(ctx, actor, project); err != nil {
		return Summary{}, err
	}
	rows, err := s.d.Issues.Issues(ctx, project)
	if err != nil {
		return Summary{}, err
	}
	var sum Summary
	byStatus, byType, byPrio := map[string]*Count{}, map[string]*Count{}, map[string]*Count{}
	loads := map[string]*Load{}
	bump := func(m map[string]*Count, k string, p int) {
		if m[k] == nil {
			m[k] = &Count{Key: k}
		}
		m[k].Count++
		m[k].Points += p
	}
	for _, r := range rows {
		p := 0
		if r.Points != nil {
			p = *r.Points
		}
		sum.Total++
		sum.TotalPoints += p
		bump(byStatus, r.Status, p)
		bump(byType, r.Type, p)
		bump(byPrio, r.Priority, p)
		if r.Assignee == "" {
			sum.Unassigned++
		}
		if r.Status == "done" {
			sum.Done++
			continue
		}
		sum.Open++
		sum.OpenPoints += p
		if loads[r.Assignee] == nil {
			loads[r.Assignee] = &Load{UserID: r.Assignee}
		}
		loads[r.Assignee].Count++
		loads[r.Assignee].Points += p
	}
	sum.ByStatus, sum.ByType, sum.ByPriority = ordered(statusOrder, byStatus), ordered(typeOrder, byType), ordered(priorityOrder, byPrio)
	for id, l := range loads {
		if id == "" {
			l.Name = "Unassigned"
		} else if l.Name, err = s.d.Users.DisplayName(ctx, id); err != nil {
			return Summary{}, err
		}
		sum.Workload = append(sum.Workload, *l)
	}
	sort.Slice(sum.Workload, func(i, j int) bool {
		a, b := sum.Workload[i], sum.Workload[j]
		if (a.UserID == "") != (b.UserID == "") {
			return b.UserID == "" // "Unassigned" last
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Name < b.Name
	})
	entries, err := s.d.Repo.ForProject(ctx, project)
	if err != nil {
		return Summary{}, err
	}
	sum.Cycle = domain.CycleTimes(entries, s.d.Now(), 30*24*time.Hour)
	return sum, nil
}

func (s *Service) CreatedVsResolved(ctx context.Context, actor, project string, days int) ([]domain.Day, error) {
	if err := s.d.Access.Authorize(ctx, actor, project); err != nil {
		return nil, err
	}
	entries, err := s.d.Repo.ForProject(ctx, project)
	if err != nil {
		return nil, err
	}
	return domain.CreatedVsResolved(entries, s.d.Now(), days), nil
}

type Burndown struct {
	Sprint SprintInfo
	Report domain.BurndownReport
}

// Burndown covers the sprint from its start until now (or its completion).
func (s *Service) Burndown(ctx context.Context, actor, sprintID string) (Burndown, error) {
	sp, err := s.d.Sprints.Sprint(ctx, sprintID)
	if err != nil {
		return Burndown{}, err
	}
	if err := s.d.Access.Authorize(ctx, actor, sp.Project); err != nil {
		return Burndown{}, ErrSprintNotFound // do not reveal other projects' sprints
	}
	if sp.Start.IsZero() {
		return Burndown{Sprint: sp}, nil // planned: nothing to burn yet
	}
	entries, err := s.d.Repo.ForProject(ctx, sp.Project)
	if err != nil {
		return Burndown{}, err
	}
	until := s.d.Now()
	if !sp.Completed.IsZero() {
		until = sp.Completed
	}
	return Burndown{Sprint: sp, Report: domain.Burndown(entries, sp.ID, sp.Start, sp.End, until)}, nil
}

// Velocity reports the last seven closed sprints, oldest first.
func (s *Service) Velocity(ctx context.Context, actor, project string) ([]domain.SprintVelocity, error) {
	if err := s.d.Access.Authorize(ctx, actor, project); err != nil {
		return nil, err
	}
	closed, err := s.d.Sprints.ClosedSprints(ctx, project, 7)
	if err != nil {
		return nil, err
	}
	windows := make([]domain.SprintWindow, 0, len(closed))
	for i := len(closed) - 1; i >= 0; i-- {
		c := closed[i]
		windows = append(windows, domain.SprintWindow{ID: c.ID, Name: c.Name, Start: c.Start, End: c.Completed})
	}
	entries, err := s.d.Repo.ForProject(ctx, project)
	if err != nil {
		return nil, err
	}
	return domain.Velocity(entries, windows), nil
}
