package domain

import (
	"fmt"
	"strings"
	"time"
)

// DigestHour is the local hour the daily digest goes out.
const DigestHour = 8

// DigestDue tells whether the user's daily digest is due at now: from DigestHour in their time zone (UTC when
// unset or unknown), once per local day. day names that local day (part of the send's idempotency key).
func DigestDue(now time.Time, tz string, last time.Time) (day string, due bool) {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	local := now.In(loc)
	if local.Hour() < DigestHour {
		return "", false
	}
	today := local.Format(time.DateOnly)
	if !last.IsZero() && last.In(loc).Format(time.DateOnly) == today {
		return "", false
	}
	return today, true
}

// RenderDigest writes the digest email: one line and link per unread notification, newest first as given.
func RenderDigest(notes []Notification, link func(Notification) string, publicURL string) (subject, text string) {
	if len(notes) == 1 {
		subject = "Kyber: 1 unread notification"
	} else {
		subject = fmt.Sprintf("Kyber: %d unread notifications", len(notes))
	}
	var b strings.Builder
	b.WriteString("Here is what happened while you were away.\n\n")
	for _, n := range notes {
		s, _ := Render(n, "")
		b.WriteString("• " + s + "\n  " + link(n) + "\n")
	}
	b.WriteString("\nYou get this digest once a day. Change it in your settings: " + strings.TrimRight(publicURL, "/") + "/settings\n")
	return subject, b.String()
}
