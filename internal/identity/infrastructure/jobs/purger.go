// Package jobs runs the identity context's background maintenance on a schedule.
package jobs

import (
	"context"
	"time"

	"github.com/bakhod1r/jitterx"
)

// Purger removes expired sessions and login codes; it returns how many rows went.
type Purger interface {
	PurgeExpiredSessions(ctx context.Context) (int, error)
}

// RunSessionPurger purges on a jittered interval until ctx is cancelled, so replicas started
// together do not all hit the database at the same moment. Failures are reported and retried.
func RunSessionPurger(ctx context.Context, p Purger, every time.Duration, report func(int, error)) {
	_ = jitterx.Every(ctx, every, nil, func(ctx context.Context) error {
		report(p.PurgeExpiredSessions(ctx))
		return nil // a failed purge is reported and retried next tick, never fatal
	})
}
