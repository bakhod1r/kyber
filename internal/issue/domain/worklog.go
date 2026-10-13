package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidDuration = errors.New("invalid duration: use w, d, h and m, e.g. 1d 4h 30m")
	ErrInvalidWorklog  = errors.New("invalid worklog")
	ErrWorklogNotFound = errors.New("worklog not found")
)

// Seconds is an amount of working time. A working day is 8 hours and a week 5 days (Jira).
type Seconds int64

const (
	minute Seconds = 60
	hour           = 60 * minute
	day            = 8 * hour
	week           = 5 * day

	maxDuration    = 52 * week
	maxWorklogText = 2000
)

var durationRe = regexp.MustCompile(`^(?:(\d+)w)?\s*(?:(\d+)d)?\s*(?:(\d+)h)?\s*(?:(\d+)m)?$`)

// ParseDuration reads Jira-style durations: "1w 2d 3h 30m"; a bare number is minutes.
func ParseDuration(s string) (Seconds, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		s += "m"
	}
	m := durationRe.FindStringSubmatch(s)
	if s == "" || m == nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidDuration, s)
	}
	var total Seconds
	for i, unit := range []Seconds{week, day, hour, minute} {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.ParseInt(m[i+1], 10, 64)
		if err != nil || n > int64(maxDuration/unit) {
			return 0, fmt.Errorf("%w: %q", ErrInvalidDuration, s)
		}
		total += Seconds(n) * unit
	}
	if total <= 0 || total > maxDuration {
		return 0, fmt.Errorf("%w: %q", ErrInvalidDuration, s)
	}
	return total, nil
}

// String formats like Jira: "1w 2d 3h 4m" ("0m" for nothing).
func (s Seconds) String() string {
	if s <= 0 {
		return "0m"
	}
	var parts []string
	for _, u := range []struct {
		size Seconds
		name string
	}{{week, "w"}, {day, "d"}, {hour, "h"}, {minute, "m"}} {
		if n := s / u.size; n > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", n, u.name))
			s -= n * u.size
		}
	}
	return strings.Join(parts, " ")
}

// TimeTracking is an issue's time: the original estimate, what remains and what was logged.
type TimeTracking struct {
	Original, Remaining, Spent Seconds
}

type WorklogID string

// Worklog is time logged on an issue (Jira's Log work).
type Worklog struct {
	ID      WorklogID `json:"id"`
	Issue   IssueID   `json:"issue_id"`
	Key     IssueKey  `json:"key"`
	Author  UserID    `json:"author"`
	Started time.Time `json:"started"`
	Seconds Seconds   `json:"seconds"`
	Comment string    `json:"comment"`
}

// IssueWorkLogged / IssueWorklogRemoved feed the worklog list (a projection) and reports.
type IssueWorkLogged struct{ Worklog }

type IssueWorklogRemoved struct {
	Worklog
	By UserID `json:"by"`
}

func (IssueWorkLogged) EventName() string     { return "issue.work_logged" }
func (IssueWorklogRemoved) EventName() string { return "issue.worklog_removed" }

func (i *Issue) TimeTracking() TimeTracking { return i.time }

// EstimateTime sets the original estimate; while nothing is logged the remaining estimate follows.
func (i *Issue) EstimateTime(original Seconds) {
	i.time.Original = max(original, 0)
	if i.time.Spent == 0 {
		i.time.Remaining = i.time.Original
	}
}

// EstimateRemaining sets how much work is left.
func (i *Issue) EstimateRemaining(remaining Seconds) { i.time.Remaining = max(remaining, 0) }

// LogWork records time spent; the remaining estimate shrinks by it, never below zero.
func (i *Issue) LogWork(id WorklogID, author UserID, started time.Time, spent Seconds, comment string) (Worklog, error) {
	if spent <= 0 || spent > maxDuration {
		return Worklog{}, ErrInvalidDuration
	}
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > maxWorklogText {
		return Worklog{}, fmt.Errorf("%w: comment is longer than %d characters", ErrInvalidWorklog, maxWorklogText)
	}
	w := Worklog{ID: id, Issue: i.id, Key: i.key, Author: author, Started: started.UTC(), Seconds: spent, Comment: comment}
	i.time.Spent += spent
	i.time.Remaining = max(i.time.Remaining-spent, 0)
	i.record(IssueWorkLogged{w})
	return w, nil
}

// RemoveWorklog takes a worklog back: its time leaves the spent total and returns to remaining.
func (i *Issue) RemoveWorklog(w Worklog, by UserID) {
	i.time.Spent = max(i.time.Spent-w.Seconds, 0)
	i.time.Remaining += w.Seconds
	i.record(IssueWorklogRemoved{Worklog: w, By: by})
}

// WorklogRepository is the worklog list of issues, projected from the events above.
type WorklogRepository interface {
	// Add and Remove apply issue.work_logged / issue.worklog_removed; both are idempotent.
	Add(ctx context.Context, w Worklog) error
	Remove(ctx context.Context, id WorklogID) error
	ByID(ctx context.Context, id WorklogID) (Worklog, error)
	// ForIssue lists the issue's worklogs, oldest first.
	ForIssue(ctx context.Context, issue IssueID) ([]Worklog, error)
}
