package telegram_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/adapter/telegram"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

const bot = "123456:ABC-test-token"

var now = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

// sign is an independent implementation of https://core.telegram.org/widgets/login#checking-authorization.
func sign(v url.Values, token string) url.Values {
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + v.Get(k)
	}
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	out := url.Values{}
	for k := range v {
		out.Set(k, v.Get(k))
	}
	out.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return out
}

func login(at time.Time) url.Values {
	return url.Values{"id": {"42"}, "first_name": {"Bobur"}, "last_name": {"Karimov"}, "username": {"bobur"},
		"photo_url": {"https://t.me/i/userpic/320/x.jpg"}, "auth_date": {strconv.FormatInt(at.Unix(), 10)}}
}

func TestVerify(t *testing.T) {
	v := telegram.NewVerifier(bot, func() time.Time { return now })
	p, err := v.Verify(sign(login(now.Add(-time.Minute)), bot))
	if err != nil {
		t.Fatal(err)
	}
	if p.Provider != domain.ProviderTelegram || p.Subject != "42" || p.Name != "Bobur Karimov" || p.Email != "" || p.EmailVerified {
		t.Fatalf("profile = %+v", p)
	}
	only := sign(url.Values{"id": {"7"}, "username": {"solo"}, "auth_date": {strconv.FormatInt(now.Unix(), 10)}}, bot)
	if p, err := v.Verify(only); err != nil || p.Name != "solo" {
		t.Fatalf("username fallback = %+v %v", p, err)
	}

	tampered := sign(login(now), bot)
	tampered.Set("id", "1")
	foreign := sign(login(now), "999:other-bot")
	missingHash := login(now)
	badHash := sign(login(now), bot)
	badHash.Set("hash", "zz")
	noID := sign(url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}}, bot)
	badDate := sign(url.Values{"id": {"1"}, "auth_date": {"yesterday"}}, bot)
	cases := map[string]url.Values{
		"tampered field":   tampered,
		"other bot's hash": foreign,
		"missing hash":     missingHash,
		"non-hex hash":     badHash,
		"missing id":       noID,
		"bad auth_date":    badDate,
		"replayed (stale)": sign(login(now.Add(-25*time.Hour)), bot),
		"from the future":  sign(login(now.Add(10*time.Minute)), bot),
	}
	for name, values := range cases {
		if _, err := v.Verify(values); !errors.Is(err, telegram.ErrInvalidLogin) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestDisabled(t *testing.T) {
	v := telegram.NewVerifier("", time.Now)
	if v.Enabled() {
		t.Fatal("no token must disable Telegram login")
	}
	if _, err := v.Verify(sign(login(now), "")); !errors.Is(err, telegram.ErrInvalidLogin) {
		t.Fatalf("err = %v", err)
	}
	if !telegram.NewVerifier(bot, time.Now).Enabled() {
		t.Fatal("token enables it")
	}
}
