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
	link := fmt.Sprintf("%s/projects/%s?issue=%s", s.d.PublicURL, url.PathEscape(project), url.QueryEscape(n.IssueKey))
	if n.ConversationID != "" {
		link = fmt.Sprintf("%s/chat/%s", s.d.PublicURL, url.PathEscape(n.ConversationID))
	}
	subject, text := domain.Render(n, link)
	for _, ch := range []domain.Channel{domain.ChannelEmail, domain.ChannelTelegram} {
		if !slices.Contains(s.d.Channels, ch) || !prefs.Wants(ch) {
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
	return s.d.Prefs.SetPrefs(ctx, domain.UserID(user), p)
}
