package domain

import "fmt"

// Render writes the email subject and the message text (email body and Telegram message) for a
// notification; link opens the issue (or the conversation) in Kyber.
func Render(n Notification, link string) (subject, text string) {
	if n.Kind == KindReminder {
		subject = "Reminder: " + n.Excerpt
		return subject, "⏰ Reminder about a saved message:\n\n" + n.Excerpt + "\n\n" + link
	}
	if n.ConversationID != "" {
		where := "a direct message"
		if n.ConversationName != "" {
			where = "#" + n.ConversationName
		}
		subject = fmt.Sprintf("%s mentioned you in %s", n.ActorName, where)
		if n.Kind == KindMessage {
			subject = fmt.Sprintf("%s wrote in %s", n.ActorName, where)
		}
		return subject, subject + ":\n\n" + n.Excerpt + "\n\n" + link
	}
	switch n.Kind {
	case KindMeeting:
		subject = n.IssueTitle + " — " + n.Excerpt
		if n.Project != "" {
			subject = fmt.Sprintf("[%s] %s", n.Project, subject)
		}
		who := ""
		if n.ActorName != "" {
			who = n.ActorName + ": "
		}
		text = fmt.Sprintf("📅 %s«%s»\n%s", who, n.IssueTitle, n.Excerpt)
	case KindAnnouncement:
		subject = "Announcement: " + n.IssueTitle
		if n.Project != "" {
			subject = fmt.Sprintf("[%s] %s", n.Project, subject)
		}
		text = fmt.Sprintf("📣 %s announced «%s».", n.ActorName, n.IssueTitle)
		if n.Excerpt != "" {
			text = fmt.Sprintf("📣 %s announced «%s»:\n\n%s", n.ActorName, n.IssueTitle, n.Excerpt)
		}
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
