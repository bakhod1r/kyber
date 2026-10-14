package domain

import (
	"errors"
	"time"
)

// ErrInvalidTimeZone: not an IANA time zone name.
var ErrInvalidTimeZone = errors.New("time zone must be an IANA name such as Europe/Berlin")

// CheckTimeZone accepts an IANA time zone name, or "" (unknown: times are shown in UTC).
func CheckTimeZone(zone string) error {
	if zone == "" {
		return nil
	}
	if zone == "Local" || len(zone) > 64 {
		return ErrInvalidTimeZone
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return ErrInvalidTimeZone
	}
	return nil
}

// location is the recipient's zone and its label; UTC when unknown.
func location(zone string) (*time.Location, string) {
	if CheckTimeZone(zone) == nil && zone != "" {
		loc, _ := time.LoadLocation(zone)
		return loc, zone
	}
	return time.UTC, "UTC"
}

// MeetingText describes a meeting event for its notification, in the recipient's time zone
// (an IANA name) when known, else in UTC.
func MeetingText(event string, start, end time.Time, zone string) string {
	loc, label := location(zone)
	start, end = start.In(loc), end.In(loc)
	when := start.Format("Mon 2 Jan, 15:04") + "–" + end.Format("15:04") + " " + label
	switch event {
	case "meeting.scheduled":
		return when
	case "meeting.rescheduled":
		return "Moved to " + when
	case "meeting.cancelled":
		return "Cancelled — was " + when
	case "meeting.reminder":
		return "Starts at " + start.Format("15:04") + " " + label
	}
	return ""
}
