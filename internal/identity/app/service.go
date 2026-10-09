// Package app holds the Identity context use cases.
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/bakhod1r/jitterx"

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
	limiter  LoginLimiter
	external domain.ExternalIdentities // nil = Google/Telegram sign-in disabled
}

// Option customises the service.
type Option func(*Service)

// WithLoginLimiter replaces the in-process limiter (e.g. with guard's Redis limiter).
func WithLoginLimiter(l LoginLimiter) Option { return func(s *Service) { s.limiter = l } }

func NewService(users domain.Users, sessions domain.Sessions, hasher domain.PasswordHasher, clock Clock, newID func() string, opts ...Option) *Service {
	dummy, _ := hasher.Hash("kyber-dummy-password")
	s := &Service{users: users, sessions: sessions, hasher: hasher, clock: clock, newID: newID, dummy: dummy}
	for _, o := range opts {
		o(s)
	}
	if s.limiter == nil {
		s.limiter = newLocalLimiter(clock)
	}
	return s
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
// At most LoginAttempts per email+IP per LoginWindow (TooManyAttemptsError). A limiter
// outage fails closed: logins are refused rather than left unthrottled.
func (s *Service) Login(ctx context.Context, rawEmail, password, ip string) (string, error) {
	allowed, retryAfter, err := s.limiter.Allow(ctx, throttleKey(rawEmail, ip))
	if err != nil {
		return "", fmt.Errorf("login rate limiter: %w", err)
	}
	if !allowed {
		return "", &TooManyAttemptsError{RetryAfter: retryAfter}
	}
	return s.login(ctx, rawEmail, password)
}

func (s *Service) login(ctx context.Context, rawEmail, password string) (string, error) {
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
	// Accounts created through Google/Telegram have no password; never verify against "".
	if u.PasswordHash() == "" {
		s.hasher.Verify(s.dummy, password)
		return "", ErrInvalidCredentials
	}
	if !s.hasher.Verify(u.PasswordHash(), password) {
		return "", ErrInvalidCredentials
	}
	return s.newSession(ctx, u.ID())
}

// newSession issues an opaque bearer token; only its SHA-256 hash is stored.
func (s *Service) newSession(ctx context.Context, user domain.UserID) (string, error) {
	var raw [32]byte
	_, _ = rand.Read(raw[:]) // never fails (Go 1.24+: crypto/rand.Read panics instead of erroring)
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	err := s.sessions.CreateSession(ctx, domain.Session{
		TokenHash: hashToken(token), UserID: user, ExpiresAt: s.clock.Now().Add(SessionTTL),
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

// PurgeExpiredSessions deletes expired sessions (run periodically).
func (s *Service) PurgeExpiredSessions(ctx context.Context) (int, error) {
	return s.sessions.DeleteExpiredSessions(ctx, s.clock.Now())
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// RunSessionPurger purges expired sessions on a jittered interval until ctx is cancelled,
// so replicas started together do not all hit the database at the same moment.
func (s *Service) RunSessionPurger(ctx context.Context, every time.Duration, report func(int, error)) {
	_ = jitterx.Every(ctx, every, nil, func(ctx context.Context) error {
		report(s.PurgeExpiredSessions(ctx))
		return nil // a failed purge is reported and retried next tick, never fatal
	})
}
