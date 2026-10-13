package domain

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"time"
)

// FieldChange is one line of an issue's history: a field and its old and new values.
type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// IssueChanged is Jira's changelog entry: who changed which fields of the issue, from what to what.
// It is written with the issue in the same transaction, so no action is ever lost.
type IssueChanged struct {
	ID    IssueID       `json:"id"`
	Key   IssueKey      `json:"key"`
	By    UserID        `json:"by"` // "" = the system (e.g. sprint completion)
	Items []FieldChange `json:"items"`
}

func (IssueChanged) EventName() string { return "issue.changed" }

// Snapshot returns the issue's current state.
func (i *Issue) Snapshot() Snapshot {
	return Snapshot{ID: i.id, Key: i.key, Title: i.title, Type: i.typ, Status: i.status, Description: i.description,
		Priority: i.priority, Assignee: i.assignee, Reporter: i.reporter, Estimate: i.estimate, Rank: i.rank, Sprint: i.sprint,
		Parent: i.parent, ParentKey: i.parentKey, Security: i.security, Resolution: i.resolution, FixVersions: slices.Clone(i.fixVersions), Time: i.time, StartDate: i.start, DueDate: i.due, Fields: maps.Clone(i.fields), Version: i.version}
}

func (i *Issue) markBaseline() {
	s := i.Snapshot()
	i.baseline = &s
}

func points(p *Points) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(p.Float(), 'f', -1, 64)
}

// Changelog lists what changed since the issue was loaded (or last saved), attributed to by.
// A new issue has none: its creation is recorded by IssueCreated.
func (i *Issue) Changelog(by UserID) (IssueChanged, bool) {
	if i.baseline == nil {
		return IssueChanged{}, false
	}
	b, a := *i.baseline, i.Snapshot()
	var items []FieldChange
	add := func(field, from, to string) {
		if from != to {
			items = append(items, FieldChange{Field: field, From: from, To: to})
		}
	}
	add("title", b.Title, a.Title)
	add("description", b.Description, a.Description)
	add("type", string(b.Type), string(a.Type))
	add("status", string(b.Status), string(a.Status))
	add("priority", string(b.Priority), string(a.Priority))
	add("assignee", string(b.Assignee), string(a.Assignee))
	add("estimate", points(b.Estimate), points(a.Estimate))
	add("sprint", string(b.Sprint), string(a.Sprint))
	add("rank", string(b.Rank), string(a.Rank))
	add("parent", keyOf(b.ParentKey), keyOf(a.ParentKey))
	add("security", b.Security, a.Security)
	add("resolution", string(b.Resolution), string(a.Resolution))
	add("fix_versions", joinVersions(b.FixVersions), joinVersions(a.FixVersions))
	add("original_estimate", durationOrEmpty(b.Time.Original), durationOrEmpty(a.Time.Original))
	add("remaining_estimate", durationOrEmpty(b.Time.Remaining), durationOrEmpty(a.Time.Remaining))
	add("time_spent", durationOrEmpty(b.Time.Spent), durationOrEmpty(a.Time.Spent))
	add("start_date", string(b.StartDate), string(a.StartDate))
	add("due_date", string(b.DueDate), string(a.DueDate))
	items = append(items, i.fieldChanges(b.Fields, a.Fields)...)
	if len(items) == 0 {
		return IssueChanged{}, false
	}
	return IssueChanged{ID: i.id, Key: i.key, By: by, Items: items}, true
}

// HistoryEntry is one line of an issue's history (Jira's History tab): who did what, when.
// Field is a changed field, or "created", "comment" or "imported".
type HistoryEntry struct {
	Event int64 // outbox id of the source event; with Seq it makes recording idempotent
	Seq   int
	Issue IssueID
	At    time.Time
	By    UserID
	Field string
	From  string
	To    string
}

// HistoryRepository stores issue history, scoped to the request's workspace.
type HistoryRepository interface {
	// Record stores entries, ignoring any (Event, Seq) already recorded.
	Record(ctx context.Context, entries ...HistoryEntry) error
	// ForIssue returns the issue's history, oldest first (At, Event, Seq).
	ForIssue(ctx context.Context, issue IssueID) ([]HistoryEntry, error)
}

func keyOf(k IssueKey) string {
	if k.Project() == "" {
		return ""
	}
	return k.String()
}

// SecurityLevel is the issue's security level ("" = visible to everyone who can browse).
func (i *Issue) SecurityLevel() string { return i.security }

// Classify restricts who can see the issue; the application checks the level exists.
func (i *Issue) Classify(level string) { i.security = level }

func durationOrEmpty(s Seconds) string {
	if s == 0 {
		return ""
	}
	return s.String()
}
