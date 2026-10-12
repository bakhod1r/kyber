package domain

import "context"

// Prefs are a user's delivery channels besides the in-app inbox. Both are on by default;
// a channel is used only when the server has it configured and the user is reachable on it.
type Prefs struct {
	Email    bool
	Telegram bool
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

// PrefsRepository stores user preferences (per user, across workspaces).
type PrefsRepository interface {
	// Prefs returns the user's preferences, DefaultPrefs when never set.
	Prefs(ctx context.Context, user UserID) (Prefs, error)
	SetPrefs(ctx context.Context, user UserID, p Prefs) error
}
