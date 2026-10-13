package domain

import "fmt"

// Render writes the email subject and the message text (email body and Telegram message) for a
// notification; link opens the issue in Kyber.
func Render(n Notification, link string) (subject, text string) {
	switch n.Kind {
	case KindAssigned:
		subject = fmt.Sprintf("[%s] %s assigned you: %s", n.IssueKey, n.ActorName, n.IssueTitle)
		text = fmt.Sprintf("%s assigned you %s «%s».", n.ActorName, n.IssueKey, n.IssueTitle)
	case KindTransitioned:
		subject = fmt.Sprintf("[%s] %s changed the status: %s", n.IssueKey, n.ActorName, n.IssueTitle)
		text = fmt.Sprintf("%s moved %s «%s»: %s.", n.ActorName, n.IssueKey, n.IssueTitle, n.Excerpt)
	case KindMentioned:
		subject = fmt.Sprintf("[%s] %s mentioned you: %s", n.IssueKey, n.ActorName, n.IssueTitle)
		text = fmt.Sprintf("%s mentioned you on %s «%s»:\n\n%s", n.ActorName, n.IssueKey, n.IssueTitle, n.Excerpt)
	default:
		subject = fmt.Sprintf("[%s] %s commented: %s", n.IssueKey, n.ActorName, n.IssueTitle)
		text = fmt.Sprintf("%s commented on %s «%s»:\n\n%s", n.ActorName, n.IssueKey, n.IssueTitle, n.Excerpt)
	}
	return subject, text + "\n\n" + link
}
