package domain

import "time"

// MeetingText describes a meeting event for its notification (times in UTC: users have no time
// zone yet).
func MeetingText(event string, start, end time.Time) string {
	start, end = start.UTC(), end.UTC()
	when := start.Format("Mon 2 Jan, 15:04") + "–" + end.Format("15:04") + " UTC"
	switch event {
	case "meeting.scheduled":
		return when
	case "meeting.cancelled":
		return "Cancelled — was " + when
	case "meeting.reminder":
		return "Starts at " + start.Format("15:04") + " UTC"
	}
	return ""
}
