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

type methodKey struct{}

// WithMethod records how the actor signed in ("password", "token", "google", "sso:<id>", …).
func WithMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, methodKey{}, method)
}

// Method returns how the actor signed in, or "".
func Method(ctx context.Context) string {
	m, _ := ctx.Value(methodKey{}).(string)
	return m
}
