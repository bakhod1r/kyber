package acl_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bakhod1r/kyber/internal/calendar/application"
	"github.com/bakhod1r/kyber/internal/calendar/infrastructure/acl"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

var errDown = errors.New("db down")

type projects struct{}

func (projects) Authorize(_ context.Context, actor projectdomain.UserID, key string, _ projectdomain.Permission) error {
	switch {
	case key != "KYB":
		return projectdomain.ErrProjectNotFound
	case actor == "viewer":
		return projectdomain.ErrForbidden
	case actor == "broken":
		return errDown
	}
	return nil
}
func (projects) MemberIDs(context.Context, string) ([]projectdomain.UserID, error) {
	return []projectdomain.UserID{"dev", "lead"}, nil
}
func (projects) WorkspaceMemberIDs(context.Context) ([]projectdomain.UserID, error) {
	return []projectdomain.UserID{"dev", "lead", "qa"}, nil
}

type workspaces struct{}

func (workspaces) Administers(_ context.Context, _, user string) (bool, error) {
	if user == "flaky" {
		return false, errDown
	}
	return user == "owner", nil
}
func (workspaces) MemberIDs(_ context.Context, ws string) ([]string, error) {
	if ws == "w-named" {
		return []string{"owner", "x"}, nil
	}
	return nil, nil
}

func site(_ context.Context, user string) (bool, error) {
	if user == "lost" {
		return false, errDown
	}
	return user == "root", nil
}

func TestTeams(t *testing.T) {
	ctx := tenant.With(context.Background(), tenant.Default)
	teams := acl.NewTeams(projects{}, workspaces{}, site)
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"member invites project", teams.InviteProject(ctx, "dev", "KYB"), nil},
		{"viewer", teams.InviteProject(ctx, "viewer", "KYB"), application.ErrForbidden},
		{"unknown project", teams.InviteProject(ctx, "dev", "NOPE"), application.ErrProjectNotFound},
		{"other error", teams.InviteProject(ctx, "broken", "KYB"), errDown},
		{"workspace owner", teams.InviteWorkspace(ctx, "owner"), nil},
		{"site admin", teams.InviteWorkspace(ctx, "root"), nil},
		{"member", teams.InviteWorkspace(ctx, "dev"), application.ErrForbidden},
		{"admin lookup", teams.InviteWorkspace(ctx, "flaky"), errDown},
		{"site lookup", teams.InviteWorkspace(ctx, "lost"), errDown},
	} {
		if !errors.Is(tc.err, tc.want) || (tc.want == nil && tc.err != nil) {
			t.Errorf("%s: %v", tc.name, tc.err)
		}
	}
	if got, _ := teams.ProjectMembers(ctx, "KYB"); !slices.Equal(got, []string{"dev", "lead"}) {
		t.Fatalf("project = %v", got)
	}
	if got, _ := teams.WorkspaceMembers(ctx); !slices.Equal(got, []string{"dev", "lead", "qa"}) {
		t.Fatalf("default workspace = %v", got)
	}
	if got, _ := teams.WorkspaceMembers(tenant.With(context.Background(), "w-named")); !slices.Equal(got, []string{"owner", "x"}) {
		t.Fatalf("named workspace = %v", got)
	}
}
