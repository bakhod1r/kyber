package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// WebhookID identifies an incoming webhook.
type WebhookID string

// Webhook lets another system (CI, monitoring) post into a channel (S81). Only the secret's SHA-256 is kept: the
// URL is shown once, when made or rotated.
type Webhook struct {
	ID           WebhookID
	Workspace    WorkspaceID
	Conversation ConversationID
	Name         string
	SecretHash   string
	CreatedBy    UserID
	CreatedAt    time.Time
}

var (
	ErrInvalidWebhook  = errors.New("a webhook has a name of 1–40 characters")
	ErrWebhookNotFound = errors.New("webhook not found")
	ErrRateLimited     = errors.New("too many messages from this webhook; slow down")
)

// HashSecret is how a webhook secret is stored and looked up.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func NewWebhook(id WebhookID, ws WorkspaceID, conv ConversationID, name, secret string, by UserID, now time.Time) (*Webhook, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > 40 {
		return nil, ErrInvalidWebhook
	}
	return &Webhook{ID: id, Workspace: ws, Conversation: conv, Name: name, SecretHash: HashSecret(secret), CreatedBy: by, CreatedAt: now.UTC()}, nil
}

// Rotate replaces the secret: the old URL stops working.
func (h *Webhook) Rotate(secret string) { h.SecretHash = HashSecret(secret) }

// Webhooks stores incoming webhooks.
type Webhooks interface {
	Add(ctx context.Context, h *Webhook) error
	Update(ctx context.Context, h *Webhook) error // the secret
	Remove(ctx context.Context, id WebhookID) error
	ByID(ctx context.Context, id WebhookID) (*Webhook, error)        // ErrWebhookNotFound
	BySecret(ctx context.Context, hash string) (*Webhook, error)     // ErrWebhookNotFound
	In(ctx context.Context, conv ConversationID) ([]*Webhook, error) // by creation
}
