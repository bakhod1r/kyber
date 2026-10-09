// Package telegram verifies Telegram Login Widget sign-ins
// (https://core.telegram.org/widgets/login#checking-authorization).
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/app"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var ErrInvalidLogin = errors.New("invalid Telegram login")

// MaxAge bounds replay of a captured widget payload (Telegram recommends checking auth_date);
// the widget posts it immediately, so minutes are plenty.
const MaxAge = 5 * time.Minute

// clockSkew tolerates a slightly fast Telegram clock.
const clockSkew = time.Minute

type Verifier struct {
	secret [32]byte
	on     bool
	now    func() time.Time
}

// NewVerifier uses the bot token from @BotFather; an empty token disables Telegram login.
func NewVerifier(botToken string, now func() time.Time) *Verifier {
	return &Verifier{secret: sha256.Sum256([]byte(botToken)), on: botToken != "", now: now}
}

func (v *Verifier) Enabled() bool { return v.on }

// Verify checks the widget's HMAC and freshness and returns the vouched profile.
func (v *Verifier) Verify(q url.Values) (app.ExternalProfile, error) {
	got, err := hex.DecodeString(q.Get("hash"))
	if !v.on || err != nil || len(got) == 0 {
		return app.ExternalProfile{}, ErrInvalidLogin
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + q.Get(k)
	}
	mac := hmac.New(sha256.New, v.secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(mac.Sum(nil), got) {
		return app.ExternalProfile{}, ErrInvalidLogin
	}
	unix, err := strconv.ParseInt(q.Get("auth_date"), 10, 64)
	if err != nil {
		return app.ExternalProfile{}, ErrInvalidLogin
	}
	age := v.now().Sub(time.Unix(unix, 0))
	if age > MaxAge || age < -clockSkew || q.Get("id") == "" {
		return app.ExternalProfile{}, ErrInvalidLogin
	}
	name := strings.TrimSpace(q.Get("first_name") + " " + q.Get("last_name"))
	if name == "" {
		name = q.Get("username")
	}
	return app.ExternalProfile{Provider: domain.ProviderTelegram, Subject: q.Get("id"), Name: name}, nil
}
