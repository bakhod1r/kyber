package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/project/domain"
)

const (
	alice domain.UserID = "u-alice"
	bob   domain.UserID = "u-bob"
	carol domain.UserID = "u-carol"
)

func newProject(t *testing.T) *domain.Project {
	t.Helper()
	p, err := domain.NewProject("p-1", domain.DefaultWorkspace, "KYB", "Kyber", alice)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewProject(t *testing.T) {
	tests := []struct {
		name, key, title string
		wantErr          error
	}{
		{"valid", "KYB", "Kyber", nil},
		{"trims name", "AB1", "  Alpha  ", nil},
		{"bad key", "kyb", "Kyber", domain.ErrInvalidKey},
		{"empty name", "KYB", "  ", domain.ErrEmptyName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := domain.NewProject("p-1", domain.DefaultWorkspace, tt.key, tt.title, alice)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (p.ID() != "p-1" || p.Key() != tt.key || p.Name() == "") {
				t.Fatalf("unexpected project %+v", p)
			}
		})
	}
}

func TestCreatorIsAdmin(t *testing.T) {
	p := newProject(t)
	if role, ok := p.RoleOf(alice); !ok || role != domain.RoleAdmin {
		t.Fatalf("creator role = %q, %v", role, ok)
	}
}

func TestParseRole(t *testing.T) {
	for _, r := range []string{"admin", "member", "viewer"} {
		if got, err := domain.ParseRole(r); err != nil || string(got) != r {
			t.Fatalf("ParseRole(%q) = %q, %v", r, got, err)
		}
	}
	if _, err := domain.ParseRole("owner"); !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("err = %v", err)
	}
}

func TestAuthorize(t *testing.T) {
	p := newProject(t)
	_ = p.SetMember(bob, domain.RoleMember)
	_ = p.SetMember(carol, domain.RoleViewer)
	tests := []struct {
		user domain.UserID
		perm domain.Permission
		want error
	}{
		{alice, domain.PermAdmin, nil},
		{bob, domain.PermWrite, nil},
		{bob, domain.PermAdmin, domain.ErrForbidden},
		{carol, domain.PermRead, nil},
		{carol, domain.PermWrite, domain.ErrForbidden},
		{"u-stranger", domain.PermRead, domain.ErrProjectNotFound}, // existence not leaked
	}
	for _, tt := range tests {
		if err := p.Authorize(tt.user, tt.perm); !errors.Is(err, tt.want) {
			t.Errorf("Authorize(%s, %v) = %v, want %v", tt.user, tt.perm, err, tt.want)
		}
	}
}

func TestSetMemberKeepsAnAdmin(t *testing.T) {
	p := newProject(t)
	if err := p.SetMember(alice, domain.RoleMember); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("err = %v, want ErrLastAdmin", err)
	}
	_ = p.SetMember(bob, domain.RoleAdmin)
	if err := p.SetMember(alice, domain.RoleViewer); err != nil {
		t.Fatalf("downgrade with another admin: %v", err)
	}
	if err := p.SetMember(bob, domain.RoleMember); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("err = %v", err)
	}
	ms := p.Members()
	if len(ms) != 2 || ms[0].UserID != alice || ms[0].Role != domain.RoleViewer || ms[1].Role != domain.RoleAdmin {
		t.Fatalf("members = %+v", ms)
	}
}

func TestNextIssueNumberAndRehydrate(t *testing.T) {
	p := newProject(t)
	for want := 1; want <= 3; want++ {
		if got := p.NextIssueNumber(); got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	}
	r := domain.Rehydrate("p-1", "w-1", "KYB", "Kyber", 41, []domain.Member{{UserID: bob, Role: domain.RoleAdmin}})
	if r.IssueSeq() != 41 || r.Workspace() != "w-1" || r.NextIssueNumber() != 42 {
		t.Fatalf("seq = %d", r.IssueSeq())
	}
	if err := r.Authorize(bob, domain.PermAdmin); err != nil {
		t.Fatal(err)
	}
}
