// Package live pushes "something changed" hints to browsers and other clients over MQTT
// (ADR-0013). A hint names
// only the project and the event (never titles, keys or other content), and is sent only to
// subscribers who may browse that project; clients then refetch through the authorized API.
package live

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

// Change is one hint: an event happened in a project of a workspace.
type Change struct {
	Workspace string `json:"-"`
	Project   string `json:"project"`
	Event     string `json:"event"`
}

// FromMessage reads the project of an outbox message from its payload ("project", or the
// project part of an issue "key"); ok is false for events outside any project.
func FromMessage(m outbox.Message) (Change, bool) {
	var p struct {
		Project string `json:"project"`
		Key     string `json:"key"`
	}
	if json.Unmarshal(m.Payload, &p) != nil {
		return Change{}, false
	}
	project := p.Project
	if i := strings.LastIndexByte(p.Key, '-'); project == "" && i > 0 {
		project = p.Key[:i]
	}
	return Change{Workspace: m.Workspace, Project: project, Event: m.Name}, project != ""
}

// Broadcaster delivers a change to the subscribers of every replica.
type Broadcaster interface {
	Broadcast(ctx context.Context, c Change) error
}

// OnEvent is the outbox handler that turns project events into hints. Hints are best effort: a
// failure is logged, never retried, so it cannot hold up or dead-letter the event.
func OnEvent(b Broadcaster, log interface {
	WarnContext(ctx context.Context, msg string, args ...any)
}) outbox.Handler {
	return func(ctx context.Context, m outbox.Message) error {
		if c, ok := FromMessage(m); ok {
			if err := b.Broadcast(ctx, c); err != nil {
				log.WarnContext(ctx, "live: broadcast failed", "event", m.Name, "err", err)
			}
		}
		return nil
	}
}
