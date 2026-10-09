// Package auth carries the authenticated actor through request contexts, so that
// bounded contexts depend on a user ID string rather than on the Identity context.
package auth

import "context"

type ctxKey struct{}

func WithActor(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// Actor returns the authenticated user ID, or "" when unauthenticated.
func Actor(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
