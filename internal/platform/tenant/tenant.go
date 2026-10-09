// Package tenant carries the request's workspace (resolved from the host, ADR-0004) in the context.
package tenant

import "context"

// Default is the workspace of single-tenant installs (workspace/domain.DefaultID).
const Default = "00000000-0000-4000-8000-000000000001"

type key struct{}

// With scopes ctx to a workspace.
func With(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, key{}, workspaceID)
}

// From returns the request's workspace; ok is false for internal calls (relay handlers, jobs)
// that are not scoped to a tenant.
func From(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(key{}).(string)
	return id, ok
}

// Workspace is the request's workspace, or the default one for unscoped calls.
func Workspace(ctx context.Context) string {
	if id, ok := From(ctx); ok {
		return id
	}
	return Default
}
