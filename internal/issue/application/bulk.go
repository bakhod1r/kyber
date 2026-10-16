package application

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// MaxBulk is how many issues one bulk change may touch.
const MaxBulk = 100

// ErrBulkInvalid: a bulk change without issues, with too many, or that changes nothing.
var ErrBulkInvalid = errors.New("a bulk change needs 1-100 issues and something to change")

// BulkChange is what to do to every selected issue; nil fields stay as they are.
type BulkChange struct {
	Status    *string  // a workflow status (each issue's transition rules apply)
	Assignee  *string  // a user id, "" = unassigned
	Sprint    *string  // a sprint id, "" = the backlog
	AddLabels []string // added to the issue's labels
}

func (c BulkChange) empty() bool {
	return c.Status == nil && c.Assignee == nil && c.Sprint == nil && len(c.AddLabels) == 0
}

// BulkResult is one issue's outcome; Err is nil when it was changed.
type BulkResult struct {
	Key string
	Err error
}

// Bulk applies one change to several issues of a project (G9). Each issue is changed through the same use
// cases as one at a time, so permissions and workflow rules apply per issue; one failure does not stop the
// others. Duplicate keys are applied once, in the order given.
func (s *Service) Bulk(ctx context.Context, actor, project string, keys []string, c BulkChange) ([]BulkResult, error) {
	var unique []string
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" && !slices.Contains(unique, k) {
			unique = append(unique, k)
		}
	}
	if len(unique) == 0 || len(unique) > MaxBulk || len(keys) > MaxBulk || c.empty() {
		return nil, ErrBulkInvalid
	}
	out := make([]BulkResult, 0, len(unique))
	for _, k := range unique {
		out = append(out, BulkResult{Key: k, Err: s.bulkOne(ctx, actor, project, k, c)})
	}
	return out, nil
}

func (s *Service) bulkOne(ctx context.Context, actor, project, key string, c BulkChange) error {
	if i := strings.LastIndexByte(key, '-'); i <= 0 || key[:i] != project {
		return domain.ErrIssueNotFound // only the project's own issues
	}
	is, err := s.Get(ctx, actor, key)
	if err != nil {
		return err
	}
	if c.Status != nil && string(is.Status()) != *c.Status {
		if is, err = s.Transition(ctx, actor, key, *c.Status, ""); err != nil {
			return err
		}
	}
	edit := EditIssue{Version: is.Version()}
	changed := false
	if c.Assignee != nil && string(is.Assignee()) != *c.Assignee {
		edit.AssigneeSet, edit.Assignee, changed = true, *c.Assignee, true
	}
	if c.Sprint != nil && string(is.Sprint()) != *c.Sprint {
		edit.SprintSet, edit.Sprint, changed = true, *c.Sprint, true
	}
	if len(c.AddLabels) > 0 {
		labels := make([]string, 0, len(is.Labels())+len(c.AddLabels))
		for _, l := range is.Labels() {
			labels = append(labels, string(l))
		}
		for _, l := range c.AddLabels {
			if !is.HasLabel(domain.Label(l)) {
				labels, changed = append(labels, l), true
			}
		}
		edit.Labels = &labels
	}
	if !changed {
		return nil
	}
	_, err = s.Edit(ctx, actor, key, edit)
	return err
}
