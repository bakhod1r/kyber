package acl_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/announcement/application"
	"github.com/bakhod1r/kyber/internal/announcement/infrastructure/acl"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
)

type projects struct{}

func (projects) Authorize(_ context.Context, actor projectdomain.UserID, key string, need projectdomain.Permission) error {
	switch {
	case key != "KYB":
		return projectdomain.ErrProjectNotFound
	case need == projectdomain.PermAdmin && actor != "lead":
		return projectdomain.ErrForbidden
	case actor == "boom":
		return errBoom
	}
	return nil
}

type workspaces struct{}

var errBoom = errors.New("boom")

func (workspaces) IsMember(_ context.Context, _, user string) (bool, error) {
	if user == "broken" {
		return false, errBoom
	}
	return user != "out", nil
}
func (workspaces) Administers(_ context.Context, _, user string) (bool, error) {
	if user == "flaky" {
		return false, errBoom
	}
	return user == "owner", nil
}

func site(_ context.Context, user string) (bool, error) {
	if user == "lost" {
		return false, errBoom
	}
	return user == "root", nil
}

func TestAccess(t *testing.T) {
	ctx := context.Background()
	a := acl.NewAccess(projects{}, workspaces{}, site)
	for _, tc := range []struct {
		name          string
		fn            func(context.Context, string, string) error
		actor, target string
		want          error
	}{
		{"member reads project", a.Read, "dev", "KYB", nil},
		{"unknown project", a.Read, "dev", "NOPE", application.ErrProjectNotFound},
		{"lead publishes", a.Publish, "lead", "KYB", nil},
		{"dev may not publish", a.Publish, "dev", "KYB", application.ErrForbidden},
		{"other errors pass", a.Read, "boom", "KYB", errBoom},
		{"workspace member reads", a.Read, "dev", "", nil},
		{"outsider", a.Read, "out", "", application.ErrProjectNotFound},
		{"membership failure", a.Read, "broken", "", errBoom},
		{"workspace owner publishes", a.Publish, "owner", "", nil},
		{"site admin publishes", a.Publish, "root", "", nil},
		{"member may not publish", a.Publish, "dev", "", application.ErrForbidden},
		{"outsider may not publish", a.Publish, "out", "", application.ErrProjectNotFound},
		{"admin lookup failure", a.Publish, "flaky", "", errBoom},
		{"site lookup failure", a.Publish, "lost", "", errBoom},
	} {
		if err := tc.fn(ctx, tc.actor, tc.target); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
