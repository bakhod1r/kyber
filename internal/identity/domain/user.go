// Package domain is the Identity context: users, credentials and sessions.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bakhod1r/emailx"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrWeakPassword    = errors.New("password must be 10-128 characters")
	ErrEmptyName       = errors.New("name must not be empty")
	ErrInvalidName     = errors.New("name must be one line of at most 100 characters")
	ErrEmailTaken      = errors.New("email already registered")
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("session not found")
)

// Email is a normalised (trimmed, lower-cased) address.
type Email string

// Syntax is checked by emailx; the address is kept as typed (only trimmed and
// lower-cased), so provider-specific rewriting such as Gmail dot removal never applies.
func ParseEmail(s string) (Email, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 254 || strings.ContainsAny(s, "<> \t\r\n") || !emailx.IsValid(s) {
		return "", ErrInvalidEmail
	}
	return Email(s), nil
}

func (e Email) String() string { return string(e) }

// ValidatePassword enforces length bounds; the upper bound protects the hasher from DoS.
func ValidatePassword(p string) error {
	if n := len([]rune(p)); n < 10 || n > 128 {
		return ErrWeakPassword
	}
	return nil
}

type UserID string

type User struct {
	id           UserID
	email        Email
	name         string
	passwordHash string
	inactive     bool // deprovisioned (SCIM): cannot sign in, tokens stop working
}

func NewUser(id UserID, email Email, name, passwordHash string) (*User, error) {
	u := &User{id: id, email: email, passwordHash: passwordHash}
	if err := u.Rename(name); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *User) ID() UserID           { return u.id }
func (u *User) Email() Email         { return u.email }
func (u *User) Name() string         { return u.name }
func (u *User) PasswordHash() string { return u.passwordHash }

// Active reports whether the user may sign in; SCIM deprovisioning deactivates instead of deleting,
// so the user's issues and comments keep their author.
func (u *User) Active() bool { return !u.inactive }
func (u *User) Deactivate()  { u.inactive = true }
func (u *User) Activate()    { u.inactive = false }

// Rename changes the display name under the same rules as NewUser.
func (u *User) Rename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyName
	}
	if strings.ContainsAny(name, "\r\n\t") || utf8.RuneCountInString(name) > 100 {
		return ErrInvalidName
	}
	u.name = name
	return nil
}

// RehydrateUser rebuilds a stored user (repositories only).
func RehydrateUser(id UserID, email Email, name, passwordHash string, active bool) *User {
	return &User{id: id, email: email, name: name, passwordHash: passwordHash, inactive: !active}
}

// Session binds a hashed bearer token to a user until ExpiresAt.
type Session struct {
	TokenHash []byte
	UserID    UserID
	ExpiresAt time.Time
	// Method is how the user signed in: MethodPassword, or the external provider ("google",
	// "sso:<connection id>", …). A workspace that requires SSO checks it (ADR-0018 §5).
	Method string
}

// Sign-in methods besides external providers.
const (
	MethodPassword = "password"
	MethodToken    = "token" // a personal access or OAuth token (no browser sign-in)
)

type Users interface {
	Create(ctx context.Context, u *User) error // ErrEmailTaken
	ByEmail(ctx context.Context, e Email) (*User, error)
	ByID(ctx context.Context, id UserID) (*User, error)
}

type Sessions interface {
	CreateSession(ctx context.Context, s Session) error
	SessionByHash(ctx context.Context, hash []byte) (Session, error) // ErrSessionNotFound
	DeleteSession(ctx context.Context, hash []byte) error
	// DeleteExpiredSessions removes sessions expired at now and returns how many.
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int, error)
}

// PasswordHasher hashes and verifies passwords (argon2id in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) bool
}

// Directory is the provisioning view of the user store (SCIM 2.0).
type Directory interface {
	// SaveUser stores a user's name and active flag (ErrUserNotFound for an unknown id).
	SaveUser(ctx context.Context, u *User) error
	// ListUsers pages through users ordered by email; email filters exactly ("" = all).
	ListUsers(ctx context.Context, email Email, offset, limit int) (users []*User, total int, err error)
	// DeleteSessionsOf signs a user out everywhere.
	DeleteSessionsOf(ctx context.Context, id UserID) error
}
