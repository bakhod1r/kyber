// Package livehints tells live subscribers (MQTT, ADR-0013) that presence on an issue changed.
package livehints

import (
	"context"

	"github.com/bakhod1r/kyber/internal/platform/live"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// Event is the hint's event name; like every hint it names only the project, never the issue.
const Event = "presence.changed"

type Logger interface {
	WarnContext(ctx context.Context, msg string, args ...any)
}

// Hints adapts a live.Broadcaster to the Issue context's presence hints port.
type Hints struct {
	b   live.Broadcaster
	log Logger
}

func New(b live.Broadcaster, log Logger) *Hints { return &Hints{b: b, log: log} }

// PresenceChanged broadcasts best effort: a failure is logged, never returned.
func (h *Hints) PresenceChanged(ctx context.Context, project string) {
	c := live.Change{Workspace: tenant.Workspace(ctx), Project: project, Event: Event}
	if err := h.b.Broadcast(ctx, c); err != nil {
		h.log.WarnContext(ctx, "live: presence hint failed", "project", project, "err", err)
	}
}
