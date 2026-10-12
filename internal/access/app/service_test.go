package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/app"
	"github.com/bakhod1r/kyber/internal/access/domain"
)

// fakeRoles: project → user → role.
type fakeRoles map[string]map[string]string

func (f fakeRoles) RoleOf(_ context.Context, project, user string) (string, bool, error) {
	if project == "BOOM" {
		return "", false, errors.New("db down")
	}
	r, ok := f[project][user]
	return r, ok, nil
}

func svc() *app.Service {
	return app.NewService(fakeRoles{"KYB": {"lead": "admin", "dev": "member", "vic": "viewer"}})
}

func TestCan(t *testing.T) {
	ctx := context.Background()
	s := svc()
	own := &app.Issue{Reporter: "vic", Status: "todo"}
	theirs := &app.Issue{Reporter: "dev", Assignee: "dev", Status: "todo"}
	cases := []struct {
		name, actor string
		perm        domain.Permission
		on          *app.Issue
		want        error
	}{
		{"viewer browses", "vic", domain.BrowseProjects, nil, nil},
		{"viewer cannot create", "vic", domain.CreateIssues, nil, app.ErrForbidden},
		{"viewer edits own issue", "vic", domain.EditIssues, own, nil},
		{"viewer cannot edit others'", "vic", domain.EditIssues, theirs, app.ErrForbidden},
		{"member edits", "dev", domain.EditIssues, theirs, nil},
		{"member cannot administer", "dev", domain.AdministerProjects, nil, app.ErrForbidden},
		{"admin administers", "lead", domain.AdministerProjects, nil, nil},
		{"outsider is not told the project exists", "eve", domain.EditIssues, theirs, app.ErrNotFound},
		{"anonymous", "", domain.BrowseProjects, nil, app.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.Can(ctx, c.actor, "KYB", c.perm, c.on)
			if !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
	if err := s.Can(ctx, "dev", "BOOM", domain.BrowseProjects, nil); err == nil || errors.Is(err, app.ErrNotFound) {
		t.Fatalf("store failure must surface as itself: %v", err)
	}
}

// The built-in "confidential" security level: admins, the reporter and the assignee only.
func TestConfidentialLevel(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(fakeRoles{"KYB": {"lead": "admin", "dev": "member", "qa": "member", "vic": "viewer"}})
	secret := &app.Issue{Reporter: "dev", Assignee: "qa", Status: "todo", SecurityLevel: app.LevelConfidential}
	for who, want := range map[string]error{"lead": nil, "dev": nil, "qa": nil, "vic": app.ErrNotFound, "eve": app.ErrNotFound} {
		err := s.Can(ctx, who, "KYB", domain.BrowseProjects, secret)
		if !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%s browse: %v, want %v", who, err, want)
		}
	}
	// Hidden means hidden for every permission, not only browsing.
	other := &app.Issue{Reporter: "lead", Status: "todo", SecurityLevel: app.LevelConfidential}
	if err := s.Can(ctx, "dev", "KYB", domain.EditIssues, other); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("edit hidden err = %v", err)
	}
	if got := app.SecurityLevels(); len(got) != 1 || got[0].ID != app.LevelConfidential {
		t.Fatalf("levels = %+v", got)
	}
}
