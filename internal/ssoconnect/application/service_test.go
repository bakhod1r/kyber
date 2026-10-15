package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/ssoconnect/application"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/memory"
)

type admins map[string]bool

func (a admins) IsSiteAdmin(_ context.Context, u string) bool { return a[u] }

type audit struct{ actions []string }

func (a *audit) Record(_ context.Context, actor, action string, c domain.Connection) error {
	a.actions = append(a.actions, actor+" "+action+" "+c.Name)
	return nil
}

// failing wraps a repository and fails saves.
type failing struct{ domain.Connections }

func (failing) Save(context.Context, *domain.Connection) error { return domain.ErrNoEncryptionKey }

func setup(repo domain.Connections) (*application.Service, *audit) {
	a := &audit{}
	return application.NewService(application.Deps{Connections: repo, Admins: admins{"boss": true}, Audit: a,
		NewID: func() string { return "c-1" }, Now: func() time.Time { return time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) }}), a
}

var okta = domain.Settings{Name: "Okta", Issuer: "https://acme.okta.com", ClientID: "id", ClientSecret: "secret", Workspace: "ignored"}

func TestManageConnections(t *testing.T) {
	ctx := context.Background()
	s, a := setup(memory.New())
	c, err := s.Create(ctx, "boss", okta)
	if err != nil || c.ID != "c-1" || c.Workspace != "" || !c.Enabled {
		t.Fatalf("create = %+v %v", c, err)
	}
	name, off := "Okta EU", false
	if c, err = s.Update(ctx, "boss", "c-1", application.Change{Name: &name, Enabled: &off}); err != nil || c.Name != name || c.Enabled {
		t.Fatalf("update = %+v %v", c, err)
	}
	on := true
	if c, _ = s.Update(ctx, "boss", "c-1", application.Change{Enabled: &on}); !c.Enabled {
		t.Fatal("not re-enabled")
	}
	if list, err := s.List(ctx, "boss"); err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	if err := s.Delete(ctx, "boss", "c-1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"boss sso.connection_created Okta", "boss sso.connection_updated Okta EU", "boss sso.connection_updated Okta EU", "boss sso.connection_deleted Okta EU"}
	if len(a.actions) != len(want) {
		t.Fatalf("audit = %v", a.actions)
	}
	for i := range want {
		if a.actions[i] != want[i] {
			t.Errorf("audit[%d] = %q, want %q", i, a.actions[i], want[i])
		}
	}
}

func TestRefusals(t *testing.T) {
	ctx := context.Background()
	s, _ := setup(memory.New())
	if _, err := s.List(ctx, "bob"); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("list: %v", err)
	}
	if _, err := s.Create(ctx, "bob", okta); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("create: %v", err)
	}
	if _, err := s.Update(ctx, "bob", "c-1", application.Change{}); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("update: %v", err)
	}
	if err := s.Delete(ctx, "bob", "c-1"); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("delete: %v", err)
	}
	bad := okta
	bad.Issuer = "http://idp.example.com"
	if _, err := s.Create(ctx, "boss", bad); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("invalid: %v", err)
	}
	if _, err := s.Update(ctx, "boss", "c-404", application.Change{}); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Errorf("update unknown: %v", err)
	}
	if err := s.Delete(ctx, "boss", "c-404"); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Errorf("delete unknown: %v", err)
	}
	if _, err := s.Create(ctx, "boss", okta); err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err := s.Update(ctx, "boss", "c-1", application.Change{Name: &empty}); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("empty name: %v", err)
	}
	noKey, _ := setup(failing{memory.New()})
	if _, err := noKey.Create(ctx, "boss", okta); !errors.Is(err, domain.ErrNoEncryptionKey) {
		t.Errorf("create without key: %v", err)
	}
	repo := memory.New()
	seeded, _ := setup(repo)
	_, _ = seeded.Create(ctx, "boss", okta)
	noKeyUpdate, _ := setup(failing{repo})
	if _, err := noKeyUpdate.Update(ctx, "boss", "c-1", application.Change{}); !errors.Is(err, domain.ErrNoEncryptionKey) {
		t.Errorf("update without key: %v", err)
	}
}

type stuck struct{ domain.Connections }

func (stuck) Delete(context.Context, domain.ConnectionID) error { return errors.New("db down") }

func TestDeleteFailure(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	s, a := setup(repo)
	_, _ = s.Create(ctx, "boss", okta)
	broken, _ := setup(stuck{repo})
	if err := broken.Delete(ctx, "boss", "c-1"); err == nil {
		t.Fatal("delete error swallowed")
	}
	if len(a.actions) != 1 {
		t.Fatalf("audited a failed delete: %v", a.actions)
	}
}
