// Package realtime publishes call state to the conversation's members (platform/realtime Hub).
package realtime

import (
	"context"

	"github.com/bakhod1r/kyber/internal/call/application"
	"github.com/bakhod1r/kyber/internal/call/domain"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

type Publisher interface {
	Publish(ws string, users []string, typ string, data any) error
}

// Notifier sends call.updated with the call rendered by render (the REST shape).
type Notifier struct {
	hub    Publisher
	render func(*domain.Call) any
}

func New(hub Publisher, render func(*domain.Call) any) Notifier {
	return Notifier{hub: hub, render: render}
}

func (n Notifier) Notify(ctx context.Context, to []string, c *domain.Call) {
	_ = n.hub.Publish(tenant.Workspace(ctx), to, "call.updated", n.render(c))
}

// Relay sends call.signal to the signal's one recipient.
func (n Notifier) Relay(ctx context.Context, s domain.Signal) {
	_ = n.hub.Publish(tenant.Workspace(ctx), []string{string(s.To)}, "call.signal", map[string]string{
		"call_id": string(s.Call), "from": string(s.From), "kind": string(s.Kind), "data": s.Data})
}

// Follow sends call.follow (the presenter's view) to the given participants.
func (n Notifier) Follow(ctx context.Context, to []string, c *domain.Call, v application.View) {
	_ = n.hub.Publish(tenant.Workspace(ctx), to, "call.follow", map[string]any{"call_id": string(c.ID), "x": v.X, "y": v.Y, "zoom": v.Zoom})
}

// React sends call.reaction (an emoji from a participant) to the others.
func (n Notifier) React(ctx context.Context, to []string, c *domain.Call, from, emoji string) {
	_ = n.hub.Publish(tenant.Workspace(ctx), to, "call.reaction", map[string]any{"call_id": string(c.ID), "from": from, "emoji": emoji})
}
