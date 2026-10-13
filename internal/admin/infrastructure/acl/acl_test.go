package acl_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/admin/domain"
	"github.com/bakhod1r/kyber/internal/admin/infrastructure/acl"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
	projectdomain "github.com/bakhod1r/kyber/internal/project/domain"
	workspacedomain "github.com/bakhod1r/kyber/internal/workspace/domain"
)

var errDown = errors.New("db down")

const named = "00000000-0000-4000-8000-0000000000aa"

type workspaces struct{ changed string }

func (*workspaces) Administers(_ context.Context, _, user string) (bool, error) {
	return user == "owner", nil
}
func (*workspaces) Members(_ context.Context, ws string) ([]workspacedomain.Member, error) {
	if ws == "broken" {
		return nil, errDown
	}
	return []workspacedomain.Member{{UserID: "owner", Role: workspacedomain.RoleOwner}}, nil
}
func (w *workspaces) ChangeRole(_ context.Context, _, user, role string) error {
	switch user {
	case "owner":
		return workspacedomain.ErrLastOwner
	case "stranger":
		return workspacedomain.ErrNotMember
	}
	if role == "boss" {
		return workspacedomain.ErrInvalidRole
	}
	w.changed = user + "=" + role
	return nil
}

type projects struct{ fail bool }

func (p projects) InWorkspace(context.Context) ([]*projectdomain.Project, error) {
	return []*projectdomain.Project{projectdomain.Rehydrate("p-1", tenant.Default, "KYB", "Kyber", 3,
		[]projectdomain.Member{{UserID: "owner", Role: projectdomain.RoleAdmin}, {UserID: "dev", Role: projectdomain.RoleMember}})}, nil
}
func (p projects) WorkspaceMemberIDs(context.Context) ([]projectdomain.UserID, error) {
	if p.fail {
		return nil, errDown
	}
	return []projectdomain.UserID{"dev", "root"}, nil
}

type users struct{}

func (users) ByID(_ context.Context, id identitydomain.UserID) (*identitydomain.User, error) {
	if id == "ghost" {
		return nil, identitydomain.ErrUserNotFound
	}
	return identitydomain.NewUser(id, identitydomain.Email(string(id)+"@x.uz"), "Name "+string(id), "h")
}

func site(_ context.Context, user string) (bool, error) {
	if user == "lost" {
		return false, errDown
	}
	return user == "root", nil
}

type ghosts struct{ projects }

func (ghosts) WorkspaceMemberIDs(context.Context) ([]projectdomain.UserID, error) {
	return []projectdomain.UserID{"ghost"}, nil
}

type lost struct{ projects }

func (lost) WorkspaceMemberIDs(context.Context) ([]projectdomain.UserID, error) {
	return []projectdomain.UserID{"lost"}, nil
}

func TestGateAndDirectory(t *testing.T) {
	def := tenant.With(context.Background(), tenant.Default)
	ws := tenant.With(context.Background(), named)
	w := &workspaces{}
	gate := acl.NewGate(w, site)
	for _, tc := range []struct {
		ctx   context.Context
		actor string
		want  bool
	}{{def, "root", true}, {def, "owner", false}, {ws, "owner", true}, {ws, "root", false}} {
		if got, _ := gate.IsAdmin(tc.ctx, tc.actor); got != tc.want {
			t.Errorf("%s = %v", tc.actor, got)
		}
	}

	dir := acl.NewDirectory(w, projects{}, users{}, site)
	ms, err := dir.Members(def)
	if err != nil || len(ms) != 2 || ms[0] != (domain.Member{ID: "dev", Name: "Name dev", Email: "dev@x.uz", Role: "member"}) || ms[1].Role != "admin" {
		t.Fatalf("default members = %+v %v", ms, err)
	}
	if ms, _ := dir.Members(ws); len(ms) != 1 || ms[0].Role != "owner" {
		t.Fatalf("named members = %+v", ms)
	}
	for name, err := range map[string]error{
		"workspace": func() error { _, err := dir.Members(tenant.With(context.Background(), "broken")); return err }(),
		"projects": func() error {
			_, err := acl.NewDirectory(w, projects{fail: true}, users{}, site).Members(def)
			return err
		}(),
		"site admins": func() error { _, err := acl.NewDirectory(w, lost{}, users{}, site).Members(def); return err }(),
		"users":       func() error { _, err := acl.NewDirectory(w, ghosts{}, users{}, site).Members(def); return err }(),
	} {
		if err == nil {
			t.Errorf("%s failure not reported", name)
		}
	}
	if err := dir.ChangeRole(def, "dev", "admin"); !errors.Is(err, domain.ErrSingleTenant) {
		t.Fatalf("default role change: %v", err)
	}
	for _, tc := range []struct {
		user, role string
		want       error
	}{{"owner", "member", domain.ErrLastOwner}, {"stranger", "admin", domain.ErrNotMember}, {"dev", "boss", domain.ErrInvalidRole}} {
		if err := dir.ChangeRole(ws, tc.user, tc.role); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.user, err)
		}
	}
	if err := dir.ChangeRole(ws, "dev", "admin"); err != nil || w.changed != "dev=admin" {
		t.Fatalf("named role change: %v %q", err, w.changed)
	}
	ps, err := acl.NewProjects(projects{}).Projects(def)
	if err != nil || len(ps) != 1 || ps[0] != (domain.Project{Key: "KYB", Name: "Kyber", Members: 2}) {
		t.Fatalf("projects = %+v %v", ps, err)
	}
}

type store struct {
	ws    string
	retry int64
}

func (s *store) DeadLettersOf(_ context.Context, ws string, _ int) ([]outbox.Message, error) {
	s.ws = ws
	return []outbox.Message{{ID: 7, Name: "issue.created", Actor: "u-1", At: time.Unix(0, 0), Attempts: 10, LastError: "boom"}}, nil
}
func (s *store) RetryOf(_ context.Context, ws string, id int64) error {
	if id == 404 {
		return outbox.ErrNotDead
	}
	s.ws, s.retry = ws, id
	return nil
}

func TestDeadLetters(t *testing.T) {
	ctx := tenant.With(context.Background(), named)
	st := &store{}
	a := acl.NewDeadLetters(st)
	dl, err := a.DeadLetters(ctx)
	if err != nil || len(dl) != 1 || dl[0].Event != "issue.created" || dl[0].Attempts != 10 || st.ws != named {
		t.Fatalf("dead letters = %+v %v", dl, err)
	}
	if err := a.Retry(ctx, 7); err != nil || st.retry != 7 || st.ws != named {
		t.Fatalf("retry: %v", err)
	}
	if err := a.Retry(ctx, 404); !errors.Is(err, domain.ErrNoDeadLetter) {
		t.Fatalf("missing: %v", err)
	}
}
