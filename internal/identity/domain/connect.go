package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// Connecting Telegram for notifications: the site shows a deep link t.me/<bot>?start=<token>;
// pressing Start sends the token to the bot, which binds the Telegram account to the user.

var ErrConnectTokenInvalid = errors.New("the Telegram connect link is not valid any more")

const (
	ConnectPrefix = "c_" // distinguishes connect links from login links on /start
	ConnectTTL    = 10 * time.Minute
)

// ConnectToken is stored by hash only: a leaked database row cannot be replayed.
type ConnectToken struct {
	Hash      string
	User      UserID
	ExpiresAt time.Time
}

// NewConnectToken returns the raw token for the link and the record to store.
func NewConnectToken(user UserID, now time.Time) (raw string, t ConnectToken) {
	var b [24]byte
	_, _ = rand.Read(b[:])
	raw = ConnectPrefix + base64.RawURLEncoding.EncodeToString(b[:]) // [A-Za-z0-9_-], 34 chars
	return raw, ConnectToken{Hash: HashConnectToken(raw), User: user, ExpiresAt: now.Add(ConnectTTL)}
}

func HashConnectToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func IsConnectPayload(payload string) bool { return strings.HasPrefix(payload, ConnectPrefix) }

// ConnectTokens stores pending connect links.
type ConnectTokens interface {
	CreateConnect(ctx context.Context, t ConnectToken) error
	// ConsumeConnect atomically uses the token: it returns the user once, then ErrConnectTokenInvalid
	// (also for unknown or expired tokens).
	ConsumeConnect(ctx context.Context, hash string, now time.Time) (UserID, error)
}
