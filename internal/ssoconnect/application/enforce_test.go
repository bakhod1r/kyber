package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/ssoconnect/application"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/memory"
)

// S107c: a workspace admin manages the workspace's own connection and may require it; members
// must then sign in through it, while owners keep their other ways in.
func TestWorkspaceConnectionsAndEnforcement(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	s, _ := withDNS(repo, dns{})
	own := okta
	own.Workspace = "w-1"
	if _, err := s.Create(ctx, "member", own); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("a member created a connection: %v", err)
	}
	if _, err := s.Create(ctx, "ws-admin", okta); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("a workspace admin created an installation-wide connection: %v", err)
	}
	c, err := s.Create(ctx, "ws-admin", own)
	if err != nil || c.Workspace != "w-1" {
		t.Fatalf("create = %+v %v", c, err)
	}
	if list, err := s.List(ctx, "ws-admin", "w-1"); err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if _, err := s.List(ctx, "ws-admin", "w-2"); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("listed another workspace: %v", err)
	}
	if _, err := s.AddDomain(ctx, "ws-admin", c.ID, "acme.uz", true); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("a workspace admin trusted a domain: %v", err)
	}
	if err := s.Check(ctx, "w-1", "member", "password"); err != nil {
		t.Fatalf("nothing is required yet: %v", err)
	}
	on := true
	if c, err = s.Update(ctx, "ws-admin", c.ID, application.Change{Enforced: &on}); err != nil || !c.Enforced {
		t.Fatalf("enforce = %+v %v", c, err)
	}
	var required *application.SSORequiredError
	if err := s.Check(ctx, "w-1", "member", "password"); !errors.As(err, &required) || required.ID != string(c.ID) || required.Name != "Okta" ||
		err.Error() != "this workspace requires single sign-on with Okta" {
		t.Fatalf("password session = %v", err)
	}
	if err := s.Check(ctx, "w-1", "member", "token"); !errors.Is(err, application.ErrSSORequired) {
		t.Fatalf("token = %v", err)
	}
	for who, method := range map[string]string{"member": domain.ProviderOf(c.ID), "ws-owner": "password"} {
		if err := s.Check(ctx, "w-1", who, method); err != nil {
			t.Errorf("%s via %s: %v", who, method, err)
		}
	}
	if err := s.Check(ctx, "w-2", "member", "password"); err != nil {
		t.Fatalf("another workspace: %v", err)
	}
	site, _ := s.Create(ctx, "boss", okta)
	if _, err := s.Update(ctx, "ws-admin", site.ID, application.Change{Name: &own.Name}); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("a workspace admin changed an installation-wide connection: %v", err)
	}
	if _, err := s.Update(ctx, "boss", site.ID, application.Change{Enforced: &on}); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Fatalf("an installation-wide connection was required: %v", err)
	}
	if list, _ := s.List(ctx, "boss", "w-1"); len(list) != 2 {
		t.Fatalf("site admins see every connection: %v", list)
	}
	if _, err := withDNSBroken().List(ctx, "boss", ""); err == nil {
		t.Fatal("list error swallowed")
	}
	if err := withDNSBroken().Check(ctx, "w-1", "member", "password"); err == nil {
		t.Fatal("check error swallowed")
	}
}

type down struct{ domain.Connections }

func (down) All(context.Context) ([]domain.Connection, error) { return nil, errors.New("db down") }
func (down) RequiredBy(context.Context, string) ([]domain.Connection, error) {
	return nil, errors.New("db down")
}

func withDNSBroken() *application.Service {
	s, _ := withDNS(down{memory.New()}, dns{})
	return s
}
