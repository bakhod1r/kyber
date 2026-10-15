// Package notifymail sends invitation emails through the Notifications context's delivery queue.
package notifymail

import (
	"context"
	"errors"

	notifyapp "github.com/bakhod1r/kyber/internal/notify/application"
	"github.com/bakhod1r/kyber/internal/project/application"
)

// Queue is the part of the Notifications context used here.
type Queue interface {
	SendEmail(ctx context.Context, workspace, to, subject, body, key string) error
}

// Mailer adapts the queue to the Project context's InvitationMailer port.
type Mailer struct{ q Queue }

func New(q Queue) Mailer { return Mailer{q: q} }

func (m Mailer) SendInvitation(ctx context.Context, workspace, to, subject, body, key string) error {
	err := m.q.SendEmail(ctx, workspace, to, subject, body, key)
	if errors.Is(err, notifyapp.ErrNoEmail) {
		return application.ErrNoMailer
	}
	return err
}
