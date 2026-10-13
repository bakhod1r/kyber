package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/issue/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// LogWork is a request to log time on an issue (Jira's Log work).
type LogWork struct {
	Spent   string    // duration, e.g. "1h 30m"
	Started time.Time // zero = now
	Comment string
}

// WorklogView is a worklog with its author's name.
type WorklogView struct {
	domain.Worklog
	AuthorName string
}

// LogWork records time on the issue; the remaining estimate shrinks by it. People who may edit
// the issue may log work on it.
func (s *Service) LogWork(ctx context.Context, actor, rawKey string, cmd LogWork) (domain.Worklog, error) {
	is, err := s.load(ctx, actor, rawKey, PermEdit)
	if err != nil {
		return domain.Worklog{}, err
	}
	spent, err := domain.ParseDuration(cmd.Spent)
	if err != nil {
		return domain.Worklog{}, err
	}
	started := cmd.Started
	if started.IsZero() {
		started = s.now()
	}
	w, err := is.LogWork(domain.WorklogID(s.newID()), domain.UserID(actor), started, spent, cmd.Comment)
	if err != nil {
		return domain.Worklog{}, err
	}
	return w, s.save(ctx, actor, is)
}

// DeleteWorklog takes a worklog back: its author, or a project admin.
func (s *Service) DeleteWorklog(ctx context.Context, actor, id string) error {
	if s.worklogs == nil {
		return domain.ErrWorklogNotFound
	}
	w, err := s.worklogs.ByID(ctx, domain.WorklogID(id))
	if err != nil {
		return err
	}
	is, err := s.load(ctx, actor, w.Key.String(), PermBrowse)
	if errors.Is(err, domain.ErrIssueNotFound) || errors.Is(err, ErrProjectNotFound) {
		return domain.ErrWorklogNotFound // do not reveal worklogs of hidden issues
	}
	if err != nil {
		return err
	}
	need := PermEdit
	if w.Author != domain.UserID(actor) {
		need = PermAdminister
	}
	if err := s.access.Can(ctx, actor, is.Key().Project(), need, is); err != nil {
		return err
	}
	is.RemoveWorklog(w, domain.UserID(actor))
	return s.save(ctx, actor, is)
}

// Worklogs lists the issue's worklogs, oldest first, for anyone who can see the issue.
func (s *Service) Worklogs(ctx context.Context, actor, rawKey string) ([]WorklogView, error) {
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
	if err != nil || s.worklogs == nil {
		return nil, err
	}
	list, err := s.worklogs.ForIssue(ctx, is.ID())
	if err != nil {
		return nil, err
	}
	out := make([]WorklogView, 0, len(list))
	for _, w := range list {
		name := "Former member"
		if w.Author != "" {
			if name, err = s.directory.DisplayName(ctx, string(w.Author)); err != nil {
				return nil, err
			}
		}
		out = append(out, WorklogView{Worklog: w, AuthorName: name})
	}
	return out, nil
}

// OnWorklogEvent projects issue.work_logged / issue.worklog_removed into the worklog list.
func (s *Service) OnWorklogEvent(ctx context.Context, m outbox.Message) error {
	var w domain.Worklog
	if err := json.Unmarshal(m.Payload, &w); err != nil || w.ID == "" || s.worklogs == nil {
		return nil // a malformed event must not block the relay
	}
	if m.Name == "issue.worklog_removed" {
		return s.worklogs.Remove(ctx, w.ID)
	}
	return s.worklogs.Add(ctx, w)
}
