package domain

import (
	"context"
	"time"
)

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed" // gave up after MaxAttempts or a permanent refusal
)

const (
	MaxAttempts = 8 // about an hour of retries
	firstWait   = 30 * time.Second
	maxWait     = 30 * time.Minute
	maxError    = 500
)

// Delivery is one POST of an event to a webhook, sent at least once by the dispatcher. Key
// ("<outbox message>/<webhook>") makes enqueueing idempotent when the relay redelivers an event.
type Delivery struct {
	ID          string
	Workspace   string
	Webhook     ID
	Event       string
	Key         string
	Body        []byte // the signed JSON document, fixed when the event is enqueued
	Status      DeliveryStatus
	Attempts    int
	NextAt      time.Time
	LastStatus  int // HTTP status of the latest attempt (0 = no response)
	LastError   string
	CreatedAt   time.Time
	DeliveredAt time.Time
}

func NewDelivery(id, workspace string, webhook ID, event, key string, body []byte, now time.Time) Delivery {
	return Delivery{ID: id, Workspace: workspace, Webhook: webhook, Event: event, Key: key, Body: body,
		Status: DeliveryPending, NextAt: now, CreatedAt: now}
}

// Delivered records a 2xx response.
func (d *Delivery) Delivered(status int, now time.Time) {
	d.Attempts++
	d.Status, d.LastStatus, d.LastError, d.DeliveredAt = DeliveryDelivered, status, "", now
}

// Failed records a failed attempt (status 0 when there was no response). It is retried with
// exponential backoff (30 s doubling, capped at 30 min) until MaxAttempts; permanent failures
// (a refused target, a deleted webhook) give up at once.
func (d *Delivery) Failed(status int, reason string, permanent bool, now time.Time) {
	d.Attempts++
	if r := []rune(reason); len(r) > maxError {
		reason = string(r[:maxError])
	}
	d.LastStatus, d.LastError = status, reason
	if permanent || d.Attempts >= MaxAttempts {
		d.Status = DeliveryFailed
		return
	}
	d.NextAt = now.Add(min(firstWait<<(d.Attempts-1), maxWait))
}

// DeliveryRepository is the delivery queue. Claim works across workspaces (the dispatcher).
type DeliveryRepository interface {
	// Enqueue stores a pending delivery; one with the same Key is ignored.
	Enqueue(ctx context.Context, d Delivery) error
	// Claim returns up to limit pending deliveries due at now and hides them for lease.
	Claim(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]Delivery, error)
	// Save stores the outcome of an attempt.
	Save(ctx context.Context, d Delivery) error
	// Recent lists a webhook's latest deliveries in the workspace of ctx, newest first.
	Recent(ctx context.Context, webhook ID, limit int) ([]Delivery, error)
}
