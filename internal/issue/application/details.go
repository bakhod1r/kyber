package application

import (
	"context"
	"fmt"
	"slices"

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
	SprintSet   bool // move to Sprint ("" = backlog)
	Sprint      string
	EstimateSet bool      // set Estimate (nil = clear)
	Estimate    *float64  // story points, one decimal
	Parent      *string   // issue key of the new parent ("" = none); nil leaves it unchanged
	Security    *string   // security level ("" = visible to everyone who can browse); nil leaves it unchanged
	Resolution  *string   // a resolved issue's resolution; nil leaves it unchanged
	FixVersions *[]string // replaces the fix versions; nil leaves them unchanged
	// OriginalEstimate / RemainingEstimate are durations like "1d 4h" ("" clears); nil leaves them.
	OriginalEstimate  *string
	RemainingEstimate *string
	// StartDate / DueDate are YYYY-MM-DD ("" clears); nil leaves them.
	StartDate *string
	DueDate   *string
	// Fields sets custom field values (field id → value, "" clears).
	Fields map[string]string
}

func (s *Service) Edit(ctx context.Context, actor, rawKey string, cmd EditIssue) (*domain.Issue, error) {
	is, err := s.load(ctx, actor, rawKey, PermEdit)
	if err != nil {
		return nil, err
	}
	if cmd.AssigneeSet {
		if err := s.access.Can(ctx, actor, is.Key().Project(), PermAssign, is); err != nil {
			return nil, err
		}
	}
	if is.Version() != cmd.Version {
		return nil, domain.ErrConcurrentModification
	}
	if cmd.AssigneeSet && cmd.Assignee != "" {
		ok, err := s.directory.IsAssignable(ctx, is.Key().Project(), cmd.Assignee)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrInvalidAssignee
		}
	}
	if cmd.SprintSet && cmd.Sprint != "" {
		if err := s.sprints.CanHold(ctx, is.Key().Project(), cmd.Sprint); err != nil {
			return nil, err
		}
	}
	original, err := optionalDuration(cmd.OriginalEstimate)
	if err != nil {
		return nil, err
	}
	remaining, err := optionalDuration(cmd.RemainingEstimate)
	if err != nil {
		return nil, err
	}
	if cmd.FixVersions != nil {
		for _, v := range *cmd.FixVersions {
			if !is.HasFixVersion(domain.VersionID(v)) { // archived versions an issue already has may stay
				if err := s.sprints.CanFix(ctx, is.Key().Project(), v); err != nil {
					return nil, err
				}
			}
		}
	}
	var estimate *domain.Points
	if cmd.EstimateSet && cmd.Estimate != nil {
		p, err := domain.ParsePoints(*cmd.Estimate)
		if err != nil {
			return nil, err
		}
		estimate = &p
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
		is.Assign(domain.UserID(cmd.Assignee), domain.UserID(actor))
	}
	if cmd.SprintSet {
		is.MoveToSprint(domain.SprintID(cmd.Sprint))
	}
	if cmd.EstimateSet {
		is.SetEstimate(estimate)
	}
	if cmd.Security != nil {
		if *cmd.Security != "" && !slices.Contains(s.levels, *cmd.Security) {
			return nil, fmt.Errorf("%q: %w", *cmd.Security, ErrInvalidSecurityLevel)
		}
		is.Classify(*cmd.Security)
	}
	if cmd.FixVersions != nil {
		ids := make([]domain.VersionID, len(*cmd.FixVersions))
		for n, v := range *cmd.FixVersions {
			ids[n] = domain.VersionID(v)
		}
		is.PlanForVersions(ids)
	}
	if err := s.setFields(ctx, is, cmd.Fields); err != nil {
		return nil, err
	}
	if original != nil {
		is.EstimateTime(*original)
	}
	if remaining != nil {
		is.EstimateRemaining(*remaining)
	}
	if cmd.StartDate != nil || cmd.DueDate != nil {
		start, due := is.Schedule()
		for _, c := range []struct {
			in  *string
			out *domain.Date
		}{{cmd.StartDate, &start}, {cmd.DueDate, &due}} {
			if c.in == nil {
				continue
			}
			d, err := domain.ParseDate(*c.in)
			if err != nil {
				return nil, err
			}
			*c.out = d
		}
		if err := is.Plan(start, due); err != nil {
			return nil, err
		}
	}
	if cmd.Resolution != nil && *cmd.Resolution != string(is.Resolution()) {
		if err := is.ChangeResolution(domain.Resolution(*cmd.Resolution)); err != nil {
			return nil, err
		}
	}
	if cmd.Parent != nil {
		parent, err := s.parentFor(ctx, is.Key().Project(), *cmd.Parent, is.Type())
		if err != nil {
			return nil, err
		}
		if err := is.SetParent(parent); err != nil {
			return nil, err
		}
	}
	return is, s.save(ctx, actor, is)
}

// CommentView is a comment plus its author's display name.
type CommentView struct {
	*domain.Comment
	AuthorName string
}

// AddComment requires the Add Comments permission.
func (s *Service) AddComment(ctx context.Context, actor, rawKey, body string) (*domain.Comment, error) {
	is, err := s.load(ctx, actor, rawKey, PermComment)
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
	is, err := s.load(ctx, actor, rawKey, PermBrowse)
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

func optionalDuration(s *string) (*domain.Seconds, error) {
	if s == nil {
		return nil, nil
	}
	var d domain.Seconds
	if *s != "" {
		v, err := domain.ParseDuration(*s)
		if err != nil {
			return nil, err
		}
		d = v
	}
	return &d, nil
}
