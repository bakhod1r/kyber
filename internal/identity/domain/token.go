package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrTokenNotFound = errors.New("personal access token not found")
	ErrInvalidToken  = errors.New("invalid personal access token")
)

// TokenPrefix starts every personal access token, so secret scanners and Authenticate can tell
// them from session tokens.
const TokenPrefix = "kpat_"

const maxTokenName = 100

type TokenID string

// PersonalToken lets scripts and AI assistants (MCP) act as its owner until it expires or is
// revoked. Only the SHA-256 hash of the secret is kept.
type PersonalToken struct {
	ID         TokenID
	UserID     UserID
	Name       string
	Hash       []byte
	CreatedAt  time.Time
	ExpiresAt  *time.Time // nil = never
	LastUsedAt *time.Time // nil = never used
}

// NewPersonalToken names a token; it may expire, but not before it is created.
func NewPersonalToken(id TokenID, user UserID, name string, hash []byte, at time.Time, expires *time.Time) (*PersonalToken, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxTokenName || strings.ContainsAny(name, "\r\n") {
		return nil, fmt.Errorf("%w: name must be one line of 1-%d characters", ErrInvalidToken, maxTokenName)
	}
	if expires != nil && !expires.After(at) {
		return nil, fmt.Errorf("%w: expiry must be in the future", ErrInvalidToken)
	}
	return &PersonalToken{ID: id, UserID: user, Name: name, Hash: hash, CreatedAt: at, ExpiresAt: expires}, nil
}

// Expired reports whether the token no longer authenticates at now.
func (t *PersonalToken) Expired(now time.Time) bool {
	return t.ExpiresAt != nil && !now.Before(*t.ExpiresAt)
}

// TouchDue reports whether LastUsedAt is stale enough to record a new use (at most once a
// minute, so authenticating does not write on every request).
func (t *PersonalToken) TouchDue(now time.Time) bool {
	return t.LastUsedAt == nil || now.Sub(*t.LastUsedAt) >= time.Minute
}

// PersonalTokens stores personal access tokens.
type PersonalTokens interface {
	CreateToken(ctx context.Context, t *PersonalToken) error
	TokenByHash(ctx context.Context, hash []byte) (*PersonalToken, error) // ErrTokenNotFound
	// TokensOf lists the user's tokens, newest first.
	TokensOf(ctx context.Context, user UserID) ([]*PersonalToken, error)
	// DeleteToken revokes one of the user's tokens (ErrTokenNotFound for anyone else's).
	DeleteToken(ctx context.Context, user UserID, id TokenID) error
	TouchToken(ctx context.Context, id TokenID, at time.Time) error
}
