// Package realtime publishes whiteboard changes and cursors to the conversation's members.
package realtime

import (
	"context"

	"github.com/bakhod1r/kyber/internal/platform/tenant"
	"github.com/bakhod1r/kyber/internal/whiteboard/domain"
)

type Publisher interface {
	Publish(ws string, users []string, typ string, data any) error
}

// Notifier sends whiteboard.ops (elements rendered by render, the REST shape) and whiteboard.cursor.
type Notifier struct {
	hub    Publisher
	render func(domain.Element) any
}

func New(hub Publisher, render func(domain.Element) any) Notifier {
	return Notifier{hub: hub, render: render}
}

func (n Notifier) Ops(ctx context.Context, to []string, board domain.WhiteboardID, by string, applied []domain.OpResult) {
	els := make([]any, len(applied))
	for i, r := range applied {
		els[i] = n.render(r.Element)
	}
	_ = n.hub.Publish(tenant.Workspace(ctx), to, "whiteboard.ops", map[string]any{"whiteboard_id": string(board), "by": by, "elements": els})
}

func (n Notifier) Cursor(ctx context.Context, to []string, board domain.WhiteboardID, by string, x, y float64) {
	_ = n.hub.Publish(tenant.Workspace(ctx), to, "whiteboard.cursor", map[string]any{"whiteboard_id": string(board), "user_id": by, "x": x, "y": y})
}
