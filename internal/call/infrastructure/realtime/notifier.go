// Package realtime publishes call state to the conversation's members (platform/realtime Hub).
package realtime

import (
	"context"

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
