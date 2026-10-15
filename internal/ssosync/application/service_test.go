package application_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssosync/application"
	"github.com/bakhod1r/kyber/internal/ssosync/domain"
	"github.com/bakhod1r/kyber/internal/ssosync/infrastructure/memory"
)

type roles struct {
	log  []string
	fail map[domain.Target]error
}

func (r *roles) Grant(_ context.Context, t domain.Target, user, role string) error {
	if err := r.fail[t]; err != nil {
		return err
	}
	r.log = append(r.log, fmt.Sprintf("grant %s %s %s", t, user, role))
	return nil
}

func (r *roles) Revoke(_ context.Context, t domain.Target, user string) error {
	if err := r.fail[t]; err != nil {
		return err
	}
	r.log = append(r.log, fmt.Sprintf("revoke %s %s", t, user))
	return nil
}

type audit struct{ lines []string }

func (a *audit) Record(_ context.Context, user, action string, t domain.Target, role string) error {
	a.lines = append(a.lines, fmt.Sprintf("%s %s %s %s", user, action, t, role))
	return nil
}

type admins map[string]bool

func (a admins) IsSiteAdmin(_ context.Context, user string) bool { return a[user] }

func setup() (*application.Service, *roles, *audit) {
	r, a, n := &roles{fail: map[domain.Target]error{}}, &audit{}, 0
	st := memory.New()
	s := application.NewService(application.Deps{Mappings: st, Grants: st, Roles: r, Audit: a, Admins: admins{"root": true},
		NewID: func() string { n++; return fmt.Sprintf("m-%d", n) }, Now: func() time.Time { return time.Date(2026, 10, 15, 9, 0, n, 0, time.UTC) },
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	return s, r, a
}

var (
	wsT = domain.Target{Scope: domain.ScopeWorkspace, Workspace: "w-1"}
	kyb = domain.Target{Scope: domain.ScopeProject, Workspace: "w-1", Project: "KYB"}
)

func TestAdminsManageMappings(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	m, err := s.Create(ctx, "root", "eng", wsT, "member")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "bob", "eng", wsT, "admin"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin create: %v", err)
	}
	if _, err := s.Create(ctx, "root", "", wsT, "admin"); !errors.Is(err, domain.ErrInvalidMapping) {
		t.Fatalf("invalid: %v", err)
	}
	if list, err := s.List(ctx, "root"); err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if _, err := s.List(ctx, "bob"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin list: %v", err)
	}
	if err := s.Delete(ctx, "bob", m.ID); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("non-admin delete: %v", err)
	}
	if err := s.Delete(ctx, "root", m.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSyncGrantsRevokesAndAudits(t *testing.T) {
	ctx := context.Background()
	s, r, a := setup()
	if err := s.Sync(ctx, "u-1", []string{"eng"}); err != nil || len(r.log) != 0 {
		t.Fatalf("no mappings: nothing happens (%v %v)", r.log, err)
	}
	_, _ = s.Create(ctx, "root", "eng", wsT, "member")
	_, _ = s.Create(ctx, "root", "eng", kyb, "member")
	_, _ = s.Create(ctx, "root", "leads", kyb, "admin")

	if err := s.Sync(ctx, "u-1", []string{"Eng", "leads"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"grant w-1 u-1 member", "grant w-1/KYB u-1 admin"}
	if !slices.Equal(r.log, want) {
		t.Fatalf("roles = %v", r.log)
	}
	if len(a.lines) != 2 || a.lines[0] != "u-1 sso.role_granted w-1 member" {
		t.Fatalf("audit = %v", a.lines)
	}
	// The same groups again: nothing to do.
	r.log, a.lines = nil, nil
	_ = s.Sync(ctx, "u-1", []string{"eng", "leads"})
	if len(r.log) != 0 || len(a.lines) != 0 {
		t.Fatalf("repeat = %v %v", r.log, a.lines)
	}
	// Left the leads group: back to member on KYB; left eng: removed from the workspace.
	_ = s.Sync(ctx, "u-1", []string{"eng"})
	if !slices.Equal(r.log, []string{"grant w-1/KYB u-1 member"}) {
		t.Fatalf("downgrade = %v", r.log)
	}
	r.log = nil
	_ = s.Sync(ctx, "u-1", nil)
	if len(r.log) != 2 || !slices.Contains(r.log, "revoke w-1 u-1") || !slices.Contains(r.log, "revoke w-1/KYB u-1") {
		t.Fatalf("revoke all = %v", r.log)
	}
	if !slices.Contains(a.lines, "u-1 sso.role_revoked w-1 member") {
		t.Fatalf("revocation audited = %v", a.lines)
	}
}

func TestSyncContinuesPastFailures(t *testing.T) {
	ctx := context.Background()
	s, r, _ := setup()
	_, _ = s.Create(ctx, "root", "eng", wsT, "member")
	_, _ = s.Create(ctx, "root", "eng", kyb, "member")
	r.fail[kyb] = errors.New("project gone")
	err := s.Sync(ctx, "u-1", []string{"eng"})
	if err == nil || !slices.Equal(r.log, []string{"grant w-1 u-1 member"}) {
		t.Fatalf("partial = %v %v", r.log, err)
	}
	delete(r.fail, kyb) // the next login retries what failed
	r.log = nil
	_ = s.Sync(ctx, "u-1", []string{"eng"})
	if !slices.Equal(r.log, []string{"grant w-1/KYB u-1 member"}) {
		t.Fatalf("retry = %v", r.log)
	}
}
