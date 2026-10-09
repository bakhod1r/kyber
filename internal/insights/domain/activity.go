// Package domain is the Insights context: an activity log (read model built from
// domain events) and pure report functions over it.
package domain

import (
	"context"
	"sort"
	"time"
)

type Kind string

const (
	KindCreated  Kind = "created"
	KindStatus   Kind = "status"   // From → To status
	KindSprint   Kind = "sprint"   // From → To sprint id ("" = backlog)
	KindEstimate Kind = "estimate" // FromPoints → ToPoints (tenths; nil = unestimated)
)

const done = "done"

// Entry is one recorded change; Event (the outbox id) makes recording idempotent.
type Entry struct {
	Event      int64
	At         time.Time
	Project    string
	Issue      string // issue id
	Kind       Kind
	From, To   string
	FromPoints *int
	ToPoints   *int
}

// Tenths converts story points in tenths to a decimal number.
func Tenths(t int) float64 { return float64(t) / 10 }

// Repository stores the activity log.
type Repository interface {
	// Record stores entries, ignoring any whose Event was already recorded for the same issue and kind.
	Record(ctx context.Context, entries ...Entry) error
	// ForProject returns the project's entries ordered by (At, Event).
	ForProject(ctx context.Context, project string) ([]Entry, error)
}

func sorted(entries []Entry) []Entry {
	out := append([]Entry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].Event < out[j].Event
	})
	return out
}

// issueState is an issue as seen by replaying the log.
type issueState struct {
	sprint string
	points int
	status string
}

type replay map[string]*issueState

func (r replay) get(id string) *issueState {
	s, ok := r[id]
	if !ok {
		s = &issueState{status: "todo"}
		r[id] = s
	}
	return s
}

func (r replay) apply(e Entry) {
	s := r.get(e.Issue)
	switch e.Kind {
	case KindCreated:
		if e.To != "" {
			s.status = e.To
		}
	case KindStatus:
		s.status = e.To
	case KindSprint:
		s.sprint = e.To
	case KindEstimate:
		s.points = 0
		if e.ToPoints != nil {
			s.points = *e.ToPoints
		}
	}
}

// remaining sums not-done members of sprint.
func (r replay) remaining(sprint string) (points, issues int) {
	for _, s := range r {
		if s.sprint == sprint && s.status != done {
			points += s.points
			issues++
		}
	}
	return
}
