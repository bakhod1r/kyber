package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidSchedule = errors.New("dates must be YYYY-MM-DD and the start date must not be after the due date")

// Date is a calendar day in ISO form (YYYY-MM-DD); "" = not set. ISO days order as strings.
type Date string

// ParseDate validates a client date ("" clears it).
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", ErrInvalidSchedule
	}
	return Date(s), nil
}

// Schedule returns the planned start and due dates (Jira's Start date and Due date; the timeline).
func (i *Issue) Schedule() (start, due Date) { return i.start, i.due }

// Plan schedules the issue between start and due; either may be "" (open-ended).
func (i *Issue) Plan(start, due Date) error {
	for _, d := range []Date{start, due} {
		if _, err := ParseDate(string(d)); err != nil {
			return err
		}
	}
	if start != "" && due != "" && start > due {
		return ErrInvalidSchedule
	}
	i.start, i.due = start, due
	return nil
}
