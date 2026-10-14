package acl_test

import (
	"context"
	"testing"

	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/acl"
)

type users map[identitydomain.UserID]*identitydomain.User

func (u users) Create(context.Context, *identitydomain.User) error { return nil }
func (u users) ByEmail(context.Context, identitydomain.Email) (*identitydomain.User, error) {
	return nil, identitydomain.ErrUserNotFound
}
func (u users) ByID(_ context.Context, id identitydomain.UserID) (*identitydomain.User, error) {
	if x, ok := u[id]; ok {
		return x, nil
	}
	return nil, identitydomain.ErrUserNotFound
}

// Placeholder addresses (Google/Telegram accounts, bots — RFC 2606 .invalid) are never mailed.
func TestContactsSkipInvalidAddresses(t *testing.T) {
	ann, _ := identitydomain.NewUser("u-1", "ann@x.uz", "Ann", "h")
	bot, _ := identitydomain.NewUser("u-2", "bot-1@bots.kyber.invalid", "CI bot", "")
	c := acl.NewContacts(users{"u-1": ann, "u-2": bot}, nil)
	if e, _ := c.Email(context.Background(), "u-1"); e != "ann@x.uz" {
		t.Fatalf("ann = %q", e)
	}
	if e, _ := c.Email(context.Background(), "u-2"); e != "" {
		t.Fatalf("bot = %q (must not be mailed)", e)
	}
}
