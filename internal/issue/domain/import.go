package domain

import (
	"fmt"
	"time"
)

// Imported is an issue brought in from another tracker, with its original history.
type Imported struct {
	Title       string
	Type        IssueType
	Reporter    UserID
	Description string
	Priority    Priority
	Assignee    UserID
	Estimate    *Points
	Status      StatusID
	CreatedAt   time.Time
	ResolvedAt  *time.Time
	ExternalKey string
}

// IssueImported is the single event of an import: it carries the original timestamps
// for reporting and deliberately replaces per-field events so nobody is notified.
type IssueImported struct {
	ID          IssueID    `json:"id"`
	Key         IssueKey   `json:"key"`
	Type        IssueType  `json:"type"`
	Title       string     `json:"title"`
	Reporter    UserID     `json:"reporter"`
	Assignee    UserID     `json:"assignee"`
	Status      StatusID   `json:"status"`
	Category    Category   `json:"status_category,omitempty"`
	Estimate    *float64   `json:"estimate"`
	ExternalKey string     `json:"external_key"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

func (IssueImported) EventName() string { return "issue.imported" }

// NewImportedIssue creates an issue directly in its imported state.
// An Imported status that is not one of the workflow's ids is read as a status category
// (what Jira exports carry) and placed in the workflow's first status of that category.
func NewImportedIssue(id IssueID, key IssueKey, in Imported, wf *Workflow) (*Issue, error) {
	title, err := NormalizeTitle(in.Title)
	if err != nil {
		return nil, err
	}
	if !wf.Has(in.Status) {
		st, ok := wf.firstIn(Category(in.Status))
		if !ok {
			return nil, fmt.Errorf("status %q: %w", in.Status, ErrUnknownStatus)
		}
		in.Status = st
	}
	if in.Priority == "" {
		in.Priority = PriorityMedium
	}
	is := &Issue{id: id, key: key, title: title, typ: in.Type, status: in.Status, description: in.Description,
		priority: in.Priority, assignee: in.Assignee, reporter: in.Reporter, estimate: in.Estimate}
	if wf.IsDone(in.Status) {
		is.resolution = ResolutionDone
	}
	var est *float64
	if in.Estimate != nil {
		f := in.Estimate.Float()
		est = &f
	}
	is.record(IssueImported{ID: id, Key: key, Type: in.Type, Title: title, Reporter: in.Reporter, Assignee: in.Assignee,
		Status: in.Status, Category: wf.CategoryOf(in.Status), Estimate: est, ExternalKey: in.ExternalKey, CreatedAt: in.CreatedAt.UTC(), ResolvedAt: in.ResolvedAt})
	return is, nil
}
