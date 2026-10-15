package domain

import (
	"context"
	"time"
)

// Prefs are a user's delivery channels besides the in-app inbox. Both are on by default;
// a channel is used only when the server has it configured and the user is reachable on it.
type Prefs struct {
	Email    bool
	Telegram bool
	// TimeZone is the user's IANA zone (sent by their browser) for times in messages; "" = UTC.
	TimeZone string
	// EmailDigest collects email into one message a day (B9) instead of one email per notification.
	EmailDigest bool
}

func DefaultPrefs() Prefs { return Prefs{Email: true, Telegram: true} }

// Wants tells whether the user receives notifications on the channel.
func (p Prefs) Wants(c Channel) bool {
	switch c {
	case ChannelEmail:
		return p.Email
	case ChannelTelegram:
		return p.Telegram
	}
	return false
}

// Immediate tells whether each notification goes out on the channel right away; with the daily digest,
// email waits for the digest.
func (p Prefs) Immediate(c Channel) bool {
	return p.Wants(c) && !(c == ChannelEmail && p.EmailDigest)
}

// WantsDigest tells whether the user gets the daily email digest.
func (p Prefs) WantsDigest() bool { return p.Email && p.EmailDigest }

// DigestSubscriber is a user with the digest on, and when their last digest went out (zero = never).
type DigestSubscriber struct {
	User     UserID
	Prefs    Prefs
	LastSent time.Time
}

// PrefsRepository stores user preferences (per user, across workspaces).
type PrefsRepository interface {
	// Prefs returns the user's preferences, DefaultPrefs when never set.
	Prefs(ctx context.Context, user UserID) (Prefs, error)
	SetPrefs(ctx context.Context, user UserID, p Prefs) error
	// DigestSubscribers lists every user with the email digest on.
	DigestSubscribers(ctx context.Context) ([]DigestSubscriber, error)
	// MarkDigestSent records when the user's digest went out.
	MarkDigestSent(ctx context.Context, user UserID, at time.Time) error
}
