package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// One-time codes delivered by the Telegram bot (KYB-S40).
const (
	OTPTTL         = 5 * time.Minute
	OTPMaxAttempts = 5
)

var (
	ErrOTPNotFound         = errors.New("login code request not found")
	ErrOTPExpired          = errors.New("login code expired")
	ErrOTPUsed             = errors.New("login code already used")
	ErrOTPNotDelivered     = errors.New("open the Telegram bot and press Start first")
	ErrOTPAlreadyDelivered = errors.New("login code already sent")
	ErrOTPWrongCode        = errors.New("wrong login code")
	ErrOTPTooManyAttempts  = errors.New("too many wrong codes; start again")
)

// OTPChallenge is one "log in with a Telegram code" attempt. The browser knows ID; the
// Telegram deep link carries Nonce; the bot binds the Telegram user and sends the code.
type OTPChallenge struct {
	ID, Nonce        string
	ExpiresAt        time.Time
	TelegramID, Name string
	CodeHash         []byte
	Attempts         int
	Used             bool
}

func NewOTPChallenge(id, nonce string, now time.Time) *OTPChallenge {
	return &OTPChallenge{ID: id, Nonce: nonce, ExpiresAt: now.Add(OTPTTL)}
}

// NewOTPCode returns a uniformly random 6-digit code.
func NewOTPCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000)) // crypto/rand never fails (Go 1.24+)
	return fmt.Sprintf("%06d", n.Int64())
}

func (c *OTPChallenge) hash(code string) []byte {
	h := sha256.Sum256([]byte(c.ID + ":" + code))
	return h[:]
}

// Deliver binds the Telegram user who pressed Start and stores the code (hashed). Once only.
func (c *OTPChallenge) Deliver(telegramID, name, code string, now time.Time) error {
	switch {
	case !now.Before(c.ExpiresAt):
		return ErrOTPExpired
	case c.CodeHash != nil:
		return ErrOTPAlreadyDelivered
	}
	c.TelegramID, c.Name, c.CodeHash = telegramID, name, c.hash(code)
	return nil
}

// Verify checks a code; every wrong code counts, and the challenge is single-use.
func (c *OTPChallenge) Verify(code string, now time.Time) error {
	switch {
	case c.Used:
		return ErrOTPUsed
	case !now.Before(c.ExpiresAt):
		return ErrOTPExpired
	case c.CodeHash == nil:
		return ErrOTPNotDelivered
	case c.Attempts >= OTPMaxAttempts:
		return ErrOTPTooManyAttempts
	}
	if subtle.ConstantTimeCompare(c.hash(code), c.CodeHash) != 1 {
		c.Attempts++
		return ErrOTPWrongCode
	}
	c.Used = true
	return nil
}

// OTPChallenges stores challenges; Update must be atomic per challenge (attempt counting).
type OTPChallenges interface {
	CreateOTP(ctx context.Context, c *OTPChallenge) error
	OTPByID(ctx context.Context, id string) (*OTPChallenge, error)       // ErrOTPNotFound
	OTPByNonce(ctx context.Context, nonce string) (*OTPChallenge, error) // ErrOTPNotFound
	// UpdateOTP loads the challenge under a lock, applies fn and saves it even when fn fails
	// (wrong codes must be counted); fn's error is returned.
	UpdateOTP(ctx context.Context, id string, fn func(*OTPChallenge) error) error
	// DeleteExpiredOTPs removes challenges expired before now and returns how many.
	DeleteExpiredOTPs(ctx context.Context, now time.Time) (int, error)
}
