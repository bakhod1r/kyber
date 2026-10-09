// Package guardlimit adapts guard's Redis sliding-window rate limiter to the
// Identity context's LoginLimiter port, so the login limit holds across replicas.
package guardlimit

import (
	"context"
	"time"

	"github.com/bakhod1r/guard/ratelimit"
	"github.com/redis/go-redis/v9"

	"github.com/bakhod1r/kyber/internal/identity/app"
)

type Limiter struct {
	l    ratelimit.Limiter
	rule ratelimit.Rule
}

func New(rdb redis.UniversalClient, prefix string) *Limiter {
	return &Limiter{
		l:    ratelimit.NewRedis(rdb, prefix),
		rule: ratelimit.Rule{Limit: app.LoginAttempts, Window: app.LoginWindow},
	}
}

func (g *Limiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	res, err := g.l.Allow(ctx, key, g.rule)
	if err != nil {
		return false, 0, err
	}
	return res.Allowed, res.RetryAfter, nil
}

var _ app.LoginLimiter = (*Limiter)(nil)
