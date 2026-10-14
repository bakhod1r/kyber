package domain

import (
	"context"
	"time"
)

// Bot is a workspace's bot user (S84): another system that posts through the normal API with its own token.
// The user itself lives in Identity (by id).
type Bot struct {
	Workspace WorkspaceID
	User      UserID
	Name      string
	CreatedBy UserID
	CreatedAt time.Time
}

// Bots lists a workspace's bots.
type Bots interface {
	Add(ctx context.Context, b *Bot) error
	In(ctx context.Context, ws WorkspaceID) ([]*Bot, error) // by creation
}
