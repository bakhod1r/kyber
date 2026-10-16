package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/bakhod1r/kyber/internal/notify/domain"
)

// Contacts finds where a user can be reached (ACL to Identity).
type Contacts interface {
	Email(ctx context.Context, user string) (string, error)
	// TelegramChat returns the user's connected Telegram chat; ok is false when not connected.
	TelegramChat(ctx context.Context, user string) (chat string, ok bool, err error)
}

// Sender delivers on one channel. Return Permanent(err) when retrying cannot help.
type Sender interface {
	Send(ctx context.Context, d domain.Delivery) error
}

type permanentError struct{ error }

func (p permanentError) Unwrap() error { return p.error }

// Permanent marks a send error as final (bad address, bot blocked): no retries.
func Permanent(err error) error { return permanentError{err} }

// IsPermanent tells whether err was marked with Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// enqueue queues the notification on every configured channel the recipient wants and can be
// reached on. Keys derive from the source event, so redelivered events never send twice.
func (s *Service) enqueue(ctx context.Context, n domain.Notification, project string) error {
	if s.d.Deliveries == nil || len(s.d.Channels) == 0 {
		return nil
	}
	prefs, err := s.d.Prefs.Prefs(ctx, n.Recipient)
	if err != nil {
		return err
	}
	link := s.link(n, project)
	subject, text := domain.Render(n, link)
	for _, ch := range []domain.Channel{domain.ChannelEmail, domain.ChannelTelegram} {
		if !slices.Contains(s.d.Channels, ch) || !prefs.Immediate(ch) {
			continue
		}
		to, ok, err := s.address(ctx, ch, string(n.Recipient))
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d/%s/%s", n.SourceEvent, n.Recipient, ch)
		d, err := domain.NewDelivery(s.d.NewID(), n.Workspace, ch, to, subject, text, key, s.d.Now())
		if err != nil {
			return err
		}
		if err := s.d.Deliveries.Enqueue(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// link opens what the notification is about in Kyber.
func (s *Service) link(n domain.Notification, project string) string {
	link := fmt.Sprintf("%s/projects/%s?issue=%s", s.d.PublicURL, url.PathEscape(project), url.QueryEscape(n.IssueKey))
	if n.ConversationID != "" {
		link = fmt.Sprintf("%s/chat/%s", s.d.PublicURL, url.PathEscape(n.ConversationID))
	}
	switch n.Kind {
	case domain.KindAnnouncement:
		link = announcementLink(s.d.PublicURL, project)
	case domain.KindMeeting:
		link = s.d.PublicURL + "/meetings"
	case domain.KindDoc:
		link = fmt.Sprintf("%s/projects/%s/docs?page=%s", s.d.PublicURL, url.PathEscape(project), url.QueryEscape(n.PageID))
	}
	return link
}

// digestSize caps the notifications listed in one digest.
const digestSize = 50

// SendDigests emails each user with the daily digest on their unread notifications since their last digest,
// from 08:00 in their time zone, once a day. The delivery key names the user's local day, so replicas running
// the job together queue one email. Returns how many digests were queued.
func (s *Service) SendDigests(ctx context.Context) (int, error) {
	if s.d.Deliveries == nil || !slices.Contains(s.d.Channels, domain.ChannelEmail) {
		return 0, nil
	}
	subs, err := s.d.Prefs.DigestSubscribers(ctx)
	if err != nil {
		return 0, err
	}
	now, sent := s.d.Now(), 0
	var errs []error // one user's failure (a removed account) never holds back the others
	for _, sub := range subs {
		queued, err := s.sendDigest(ctx, sub, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("digest for %s: %w", sub.User, err))
		}
		if queued {
			sent++
		}
	}
	return sent, errors.Join(errs...)
}

// sendDigest queues one user's digest when it is due and something is unread.
func (s *Service) sendDigest(ctx context.Context, sub domain.DigestSubscriber, now time.Time) (bool, error) {
	day, due := domain.DigestDue(now, sub.Prefs.TimeZone, sub.LastSent)
	if !due || !sub.Prefs.WantsDigest() {
		return false, nil
	}
	since := sub.LastSent
	if since.IsZero() {
		since = now.Add(-24 * time.Hour)
	}
	notes, err := s.d.Repo.UnreadSince(ctx, sub.User, since, digestSize)
	if err != nil || len(notes) == 0 {
		return false, err // nothing yet: a later run today may still have something to send
	}
	to, err := s.d.Contacts.Email(ctx, string(sub.User))
	if err != nil {
		return false, err
	}
	queued := false
	if to != "" {
		subject, text := domain.RenderDigest(notes, func(n domain.Notification) string { return s.link(n, noteProject(n)) }, s.d.PublicURL)
		d, err := domain.NewDelivery(s.d.NewID(), notes[0].Workspace, domain.ChannelEmail, to, subject, text, "digest/"+string(sub.User)+"/"+day, now)
		if err != nil {
			return false, err
		}
		if err := s.d.Deliveries.Enqueue(ctx, d); err != nil {
			return false, err
		}
		queued = true
	}
	return queued, s.d.Prefs.MarkDigestSent(ctx, sub.User, now)
}

// noteProject is the project a notification belongs to: its own, or its issue key's.
func noteProject(n domain.Notification) string {
	if n.Project != "" {
		return n.Project
	}
	return projectOf(n.IssueKey)
}

func (s *Service) address(ctx context.Context, ch domain.Channel, user string) (string, bool, error) {
	if ch == domain.ChannelTelegram {
		return s.d.Contacts.TelegramChat(ctx, user)
	}
	email, err := s.d.Contacts.Email(ctx, user)
	return email, email != "", err
}

const (
	dispatchBatch = 50
	dispatchLease = 2 * time.Minute // longer than any single send
)

// Dispatch sends one batch of due deliveries and returns how many were sent.
func (s *Service) Dispatch(ctx context.Context, senders map[domain.Channel]Sender) (int, error) {
	due, err := s.d.Deliveries.Claim(ctx, s.d.Now(), dispatchLease, dispatchBatch)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, d := range due {
		sender, ok := senders[d.Channel]
		switch {
		case !ok:
			d.Failed("no sender for channel "+string(d.Channel), false, s.d.Now())
		default:
			if err := sender.Send(ctx, d); err != nil {
				d.Failed(err.Error(), IsPermanent(err), s.d.Now())
				s.d.Log.WarnContext(ctx, "notify: delivery failed", "id", d.ID, "channel", d.Channel, "attempt", d.Attempts, "err", err)
			} else {
				d.Sent(s.d.Now())
				sent++
			}
		}
		if err := s.d.Deliveries.Save(ctx, d); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// Settings is what the user sees on the notification settings page.
type Settings struct {
	domain.Prefs
	EmailAvailable, TelegramAvailable bool // configured on this server
	TelegramConnected                 bool // the user linked a Telegram chat
}

func (s *Service) Settings(ctx context.Context, user string) (Settings, error) {
	out := Settings{EmailAvailable: slices.Contains(s.d.Channels, domain.ChannelEmail),
		TelegramAvailable: slices.Contains(s.d.Channels, domain.ChannelTelegram)}
	if s.d.Prefs == nil {
		out.Prefs = domain.DefaultPrefs()
		return out, nil
	}
	p, err := s.d.Prefs.Prefs(ctx, domain.UserID(user))
	if err != nil {
		return out, err
	}
	out.Prefs = p
	if s.d.Contacts != nil {
		_, out.TelegramConnected, err = s.d.Contacts.TelegramChat(ctx, user)
	}
	return out, err
}

func (s *Service) SetPrefs(ctx context.Context, user string, p domain.Prefs) error {
	if err := domain.CheckTimeZone(p.TimeZone); err != nil {
		return err
	}
	return s.d.Prefs.SetPrefs(ctx, domain.UserID(user), p)
}

// announcementLink opens a project's announcements, or the home page for the workspace's own.
func announcementLink(base, project string) string {
	if project == "" {
		return base + "/"
	}
	return fmt.Sprintf("%s/projects/%s/announcements", base, url.PathEscape(project))
}

// ErrNoEmail means this server has no email channel configured.
var ErrNoEmail = errors.New("email is not configured")

// SendEmail queues a one-off email (an invitation to someone without an account) on the delivery queue,
// with its retries. key makes the send idempotent. ErrNoEmail without SMTP.
func (s *Service) SendEmail(ctx context.Context, workspace, to, subject, body, key string) error {
	if s.d.Deliveries == nil || !slices.Contains(s.d.Channels, domain.ChannelEmail) {
		return ErrNoEmail
	}
	d, err := domain.NewDelivery(s.d.NewID(), workspace, domain.ChannelEmail, to, subject, body, key, s.d.Now())
	if err != nil {
		return err
	}
	return s.d.Deliveries.Enqueue(ctx, d)
}
