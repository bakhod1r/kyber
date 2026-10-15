package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/application"
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

func (f fakeRoles) Archived(_ context.Context, project string) (bool, error) {
	if project == "ARC-DOWN" {
		return false, errors.New("db down")
	}
	return project == "ARC", nil
}

// The archived check's own failure surfaces as itself, not as a denial.
func TestArchivedCheckFailure(t *testing.T) {
	s := application.NewService(fakeRoles{"ARC-DOWN": {"lead": "admin"}})
	err := s.Can(context.Background(), "lead", "ARC-DOWN", domain.EditIssues, nil)
	if err == nil || errors.Is(err, application.ErrForbidden) || errors.Is(err, application.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

// B10: in an archived project everyone keeps reading; nobody changes anything, admins included.
func TestArchivedProjectIsReadOnly(t *testing.T) {
	ctx := context.Background()
	s := application.NewService(fakeRoles{"ARC": {"lead": "admin", "dev": "member"}})
	if err := s.Can(ctx, "dev", "ARC", domain.BrowseProjects, nil); err != nil {
		t.Fatalf("browse: %v", err)
	}
	for _, perm := range []domain.Permission{domain.CreateIssues, domain.EditIssues, domain.AdministerProjects} {
		err := s.Can(ctx, "lead", "ARC", perm, nil)
		if !errors.Is(err, application.ErrForbidden) || !errors.Is(err, application.ErrArchived) {
			t.Fatalf("%s: %v", perm, err)
		}
	}
	if err := s.Can(ctx, "eve", "ARC", domain.EditIssues, nil); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("outsider still hidden: %v", err)
	}
}

func svc() *application.Service {
	return application.NewService(fakeRoles{"KYB": {"lead": "admin", "dev": "member", "vic": "viewer"}})
}

func TestCan(t *testing.T) {
	ctx := context.Background()
	s := svc()
	own := &application.Issue{Reporter: "vic", Status: "todo"}
	theirs := &application.Issue{Reporter: "dev", Assignee: "dev", Status: "todo"}
	cases := []struct {
		name, actor string
		perm        domain.Permission
		on          *application.Issue
		want        error
	}{
		{"viewer browses", "vic", domain.BrowseProjects, nil, nil},
		{"viewer cannot create", "vic", domain.CreateIssues, nil, application.ErrForbidden},
		{"viewer edits own issue", "vic", domain.EditIssues, own, nil},
		{"viewer cannot edit others'", "vic", domain.EditIssues, theirs, application.ErrForbidden},
		{"member edits", "dev", domain.EditIssues, theirs, nil},
		{"member cannot administer", "dev", domain.AdministerProjects, nil, application.ErrForbidden},
		{"admin administers", "lead", domain.AdministerProjects, nil, nil},
		{"outsider is not told the project exists", "eve", domain.EditIssues, theirs, application.ErrNotFound},
		{"anonymous", "", domain.BrowseProjects, nil, application.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.Can(ctx, c.actor, "KYB", c.perm, c.on)
			if !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
	if err := s.Can(ctx, "dev", "BOOM", domain.BrowseProjects, nil); err == nil || errors.Is(err, application.ErrNotFound) {
		t.Fatalf("store failure must surface as itself: %v", err)
	}
}

// The built-in "confidential" security level: admins, the reporter and the assignee only.
func TestConfidentialLevel(t *testing.T) {
	ctx := context.Background()
	s := application.NewService(fakeRoles{"KYB": {"lead": "admin", "dev": "member", "qa": "member", "vic": "viewer"}})
	secret := &application.Issue{Reporter: "dev", Assignee: "qa", Status: "todo", SecurityLevel: application.LevelConfidential}
	for who, want := range map[string]error{"lead": nil, "dev": nil, "qa": nil, "vic": application.ErrNotFound, "eve": application.ErrNotFound} {
		err := s.Can(ctx, who, "KYB", domain.BrowseProjects, secret)
		if !errors.Is(err, want) || (want == nil) != (err == nil) {
			t.Errorf("%s browse: %v, want %v", who, err, want)
		}
	}
	// Hidden means hidden for every permission, not only browsing.
	other := &application.Issue{Reporter: "lead", Status: "todo", SecurityLevel: application.LevelConfidential}
	if err := s.Can(ctx, "dev", "KYB", domain.EditIssues, other); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("edit hidden err = %v", err)
	}
	if got := application.SecurityLevels(); len(got) != 1 || got[0].ID != application.LevelConfidential {
		t.Fatalf("levels = %+v", got)
	}
}

type schemes struct {
	fakeTable domain.GrantTable
	err       error
	saveErr   error
}

func (s *schemes) ProjectGrants(context.Context, string) (domain.GrantTable, bool, error) {
	return s.fakeTable, s.fakeTable != nil, s.err
}
func (s *schemes) SaveProjectGrants(_ context.Context, _ string, t domain.GrantTable) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.fakeTable = t
	return nil
}
func (s *schemes) DeleteProjectGrants(context.Context, string) error {
	s.fakeTable = nil
	return nil
}

func TestProjectSchemes(t *testing.T) {
	ctx := context.Background()
	store := &schemes{}
	s := svc().WithSchemes(store)

	table, custom, err := s.ProjectScheme(ctx, "lead", "KYB")
	if err != nil || custom || len(table[domain.CreateIssues]) != 2 {
		t.Fatalf("default = %v %v %v", table, custom, err)
	}
	for who, want := range map[string]error{"dev": application.ErrForbidden, "eve": application.ErrNotFound} {
		if _, _, err := s.ProjectScheme(ctx, who, "KYB"); !errors.Is(err, want) {
			t.Errorf("read as %s: %v", who, err)
		}
		if _, err := s.SetProjectScheme(ctx, who, "KYB", table); !errors.Is(err, want) {
			t.Errorf("set as %s: %v", who, err)
		}
		if err := s.ResetProjectScheme(ctx, who, "KYB"); !errors.Is(err, want) {
			t.Errorf("reset as %s: %v", who, err)
		}
	}

	if err := s.Can(ctx, "vic", "KYB", domain.CreateIssues, nil); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("viewer creates under default: %v", err)
	}
	table[domain.CreateIssues] = append(table[domain.CreateIssues], domain.Role(domain.RoleViewer))
	if _, err := s.SetProjectScheme(ctx, "lead", "KYB", table); err != nil {
		t.Fatal(err)
	}
	if err := s.Can(ctx, "vic", "KYB", domain.CreateIssues, nil); err != nil {
		t.Fatalf("viewer creates under the project's scheme: %v", err)
	}
	if _, custom, _ := s.ProjectScheme(ctx, "lead", "KYB"); !custom {
		t.Fatal("scheme not reported as the project's own")
	}
	// Security levels still apply under a project's own scheme.
	if err := s.Can(ctx, "vic", "KYB", domain.BrowseProjects, &application.Issue{SecurityLevel: application.LevelConfidential}); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("confidential under custom scheme: %v", err)
	}

	lockout := domain.DefaultScheme().Table()
	lockout[domain.AdministerProjects] = nil
	if _, err := s.SetProjectScheme(ctx, "lead", "KYB", lockout); !errors.Is(err, application.ErrInvalidScheme) {
		t.Fatalf("lock-out err = %v", err)
	}

	if err := s.ResetProjectScheme(ctx, "lead", "KYB"); err != nil {
		t.Fatal(err)
	}
	if err := s.Can(ctx, "vic", "KYB", domain.CreateIssues, nil); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("after reset: %v", err)
	}

	// Store failures and a corrupt stored scheme surface as errors.
	store.saveErr = errors.New("disk full")
	if _, err := s.SetProjectScheme(ctx, "lead", "KYB", table); !errors.Is(err, store.saveErr) {
		t.Fatalf("save error: %v", err)
	}
	store.err = errors.New("db down")
	if err := s.Can(ctx, "lead", "KYB", domain.BrowseProjects, nil); err == nil {
		t.Fatal("store error swallowed")
	}
	store.err, store.fakeTable = nil, domain.GrantTable{domain.CreateIssues: nil}
	if err := s.Can(ctx, "lead", "KYB", domain.BrowseProjects, nil); !errors.Is(err, application.ErrInvalidScheme) {
		t.Fatalf("corrupt scheme: %v", err)
	}
}
