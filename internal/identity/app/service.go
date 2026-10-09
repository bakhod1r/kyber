// Package app holds the Identity context use cases.
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/bakhod1r/kyber/internal/identity/domain"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthenticated    = errors.New("authentication required")
)

const SessionTTL = 30 * 24 * time.Hour

type Clock interface{ Now() time.Time }

type Service struct {
	users    domain.Users
	sessions domain.Sessions
	hasher   domain.PasswordHasher
	clock    Clock
	newID    func() string
	dummy    string // hash verified for unknown users to equalise timing
}

func NewService(users domain.Users, sessions domain.Sessions, hasher domain.PasswordHasher, clock Clock, newID func() string) *Service {
	dummy, _ := hasher.Hash("kyber-dummy-password")
	return &Service{users: users, sessions: sessions, hasher: hasher, clock: clock, newID: newID, dummy: dummy}
}

type Signup struct{ Email, Name, Password string }

func (s *Service) Signup(ctx context.Context, cmd Signup) (*domain.User, error) {
	email, err := domain.ParseEmail(cmd.Email)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePassword(cmd.Password); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}
	u, err := domain.NewUser(domain.UserID(s.newID()), email, cmd.Name, hash)
	if err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login returns an opaque bearer token; only its SHA-256 hash is stored.
func (s *Service) Login(ctx context.Context, rawEmail, password string) (string, error) {
	email, err := domain.ParseEmail(rawEmail)
	if err != nil {
		s.hasher.Verify(s.dummy, password)
		return "", ErrInvalidCredentials
	}
	u, err := s.users.ByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		s.hasher.Verify(s.dummy, password)
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if !s.hasher.Verify(u.PasswordHash(), password) {
		return "", ErrInvalidCredentials
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	err = s.sessions.CreateSession(ctx, domain.Session{
		TokenHash: hashToken(token), UserID: u.ID(), ExpiresAt: s.clock.Now().Add(SessionTTL),
	})
	return token, err
}

func (s *Service) Authenticate(ctx context.Context, token string) (*domain.User, error) {
	sess, err := s.sessions.SessionByHash(ctx, hashToken(token))
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if !s.clock.Now().Before(sess.ExpiresAt) {
		return nil, ErrUnauthenticated
	}
	u, err := s.users.ByID(ctx, sess.UserID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return nil, ErrUnauthenticated
	}
	return u, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.DeleteSession(ctx, hashToken(token))
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
