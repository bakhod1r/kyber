package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrInvalidDelivery = errors.New("a delivery needs a channel (email or telegram), a recipient and a body")

// Channel is where a notification is sent besides the in-app inbox.
type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelTelegram Channel = "telegram"
)

func (c Channel) Valid() bool { return c == ChannelEmail || c == ChannelTelegram }

type DeliveryStatus string

const (
	DeliveryPending DeliveryStatus = "pending"
	DeliverySent    DeliveryStatus = "sent"
	DeliveryDead    DeliveryStatus = "dead" // gave up; kept for inspection (dead-letter)
)

const (
	MaxAttempts = 10   // about 2½ hours of retries
	MaxBody     = 3500 // fits a Telegram message (4096) with room for markup
	firstWait   = 30 * time.Second
	maxWait     = time.Hour
)

// Delivery is one message on an external channel, sent at least once by the dispatcher.
// Key (e.g. "<notification>/email") makes enqueueing idempotent.
type Delivery struct {
	ID        string
	Workspace string
	Channel   Channel
	To        string // email address or Telegram chat id
	Subject   string
	Body      string
	Key       string
	Status    DeliveryStatus
	Attempts  int
	NextAt    time.Time
	LastError string
	CreatedAt time.Time
	SentAt    time.Time
}

func NewDelivery(id, workspace string, ch Channel, to, subject, body, key string, now time.Time) (Delivery, error) {
	to, body = strings.TrimSpace(to), strings.TrimSpace(body)
	if !ch.Valid() || to == "" || body == "" {
		return Delivery{}, ErrInvalidDelivery
	}
	if r := []rune(body); len(r) > MaxBody {
		body = string(r[:MaxBody-1]) + "…"
	}
	return Delivery{ID: id, Workspace: workspace, Channel: ch, To: to, Subject: subject, Body: body, Key: key,
		Status: DeliveryPending, NextAt: now, CreatedAt: now}, nil
}

// Sent records a successful send.
func (d *Delivery) Sent(now time.Time) {
	d.Attempts++
	d.Status, d.SentAt, d.LastError = DeliverySent, now, ""
}

// Failed records a failed attempt. Temporary failures are retried with exponential backoff
// (30 s doubling, capped at 1 h); permanent ones (bad address, bot blocked) and the last
// attempt move the delivery to the dead-letter state.
func (d *Delivery) Failed(reason string, permanent bool, now time.Time) {
	d.Attempts++
	d.LastError = reason
	if permanent || d.Attempts >= MaxAttempts {
		d.Status = DeliveryDead
		return
	}
	wait := firstWait << (d.Attempts - 1)
	if wait > maxWait {
		wait = maxWait
	}
	d.NextAt = now.Add(wait)
}

// DeliveryRepository is the delivery queue. Claim is used by the dispatcher across workspaces.
type DeliveryRepository interface {
	// Enqueue stores a pending delivery; a delivery with the same Key is ignored.
	Enqueue(ctx context.Context, d Delivery) error
	// Claim returns up to limit pending deliveries due at now and hides them for lease, so
	// several dispatchers never send the same message concurrently.
	Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Delivery, error)
	// Save stores the outcome of an attempt.
	Save(ctx context.Context, d Delivery) error
}
