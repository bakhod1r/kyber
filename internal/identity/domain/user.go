// Package domain is the Identity context: users, credentials and sessions.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrWeakPassword    = errors.New("password must be 10-128 characters")
	ErrEmptyName       = errors.New("name must not be empty")
	ErrEmailTaken      = errors.New("email already registered")
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("session not found")
)

// Email is a normalised (trimmed, lower-cased) address.
type Email string

func ParseEmail(s string) (Email, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	at := strings.IndexByte(s, '@')
	if len(s) > 254 || at < 1 || at == len(s)-1 || strings.ContainsAny(s, " \t\r\n") || strings.Count(s, "@") != 1 {
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
}

func NewUser(id UserID, email Email, name, passwordHash string) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	return &User{id: id, email: email, name: name, passwordHash: passwordHash}, nil
}

func (u *User) ID() UserID           { return u.id }
func (u *User) Email() Email         { return u.email }
func (u *User) Name() string         { return u.name }
func (u *User) PasswordHash() string { return u.passwordHash }

// Session binds a hashed bearer token to a user until ExpiresAt.
type Session struct {
	TokenHash []byte
	UserID    UserID
	ExpiresAt time.Time
}

type Users interface {
	Create(ctx context.Context, u *User) error // ErrEmailTaken
	ByEmail(ctx context.Context, e Email) (*User, error)
	ByID(ctx context.Context, id UserID) (*User, error)
}

type Sessions interface {
	CreateSession(ctx context.Context, s Session) error
	SessionByHash(ctx context.Context, hash []byte) (Session, error) // ErrSessionNotFound
	DeleteSession(ctx context.Context, hash []byte) error
}

// PasswordHasher hashes and verifies passwords (argon2id in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) bool
}
