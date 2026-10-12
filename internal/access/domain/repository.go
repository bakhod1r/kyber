package domain

import "context"

// SchemeRepository stores per-project permission schemes in the workspace of ctx.
// ok=false means the project has no scheme of its own and uses the default.
type SchemeRepository interface {
	ProjectGrants(ctx context.Context, project string) (t GrantTable, ok bool, err error)
	SaveProjectGrants(ctx context.Context, project string, t GrantTable) error
	DeleteProjectGrants(ctx context.Context, project string) error
}
