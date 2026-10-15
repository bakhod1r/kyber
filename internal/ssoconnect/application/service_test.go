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

type dns map[string][]string

func (d dns) TXT(_ context.Context, name string) ([]string, error) {
	r, ok := d[name]
	if !ok {
		return nil, errors.New("no such host")
	}
	return r, nil
}

func withDNS(repo domain.Connections, records dns) (*application.Service, *audit) {
	a := &audit{}
	return application.NewService(application.Deps{Connections: repo, Admins: admins{"boss": true}, Audit: a, DNS: records,
		NewID: func() string { return "c-1" }, NewToken: func() string { return "tok" },
		Now: func() time.Time { return time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) }}), a
}

func TestDomainsAndDiscovery(t *testing.T) {
	ctx := context.Background()
	records := dns{}
	s, a := withDNS(memory.New(), records)
	if _, err := s.Create(ctx, "boss", okta); err != nil {
		t.Fatal(err)
	}
	c, err := s.AddDomain(ctx, "boss", "c-1", "acme.uz", false)
	if err != nil || c.Domains[0].Token != "tok" || c.Domains[0].Verified {
		t.Fatalf("add = %+v %v", c, err)
	}
	if _, err := s.Discover(ctx, "zara@acme.uz"); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Fatalf("an unverified domain routes: %v", err)
	}
	if _, err := s.VerifyDomain(ctx, "boss", "c-1", "acme.uz"); !errors.Is(err, domain.ErrDomainNotVerified) {
		t.Fatalf("lookup failure: %v", err)
	}
	records["acme.uz"] = []string{"kyber-verify=tok"}
	if c, err = s.VerifyDomain(ctx, "boss", "c-1", "acme.uz"); err != nil || !c.Domains[0].Verified {
		t.Fatalf("verify = %+v %v", c, err)
	}
	if got, err := s.Discover(ctx, "Zara@ACME.uz"); err != nil || got.ID != "c-1" {
		t.Fatalf("discover = %+v %v", got, err)
	}
	for _, email := range []string{"zara@other.uz", "no-at-sign"} {
		if _, err := s.Discover(ctx, email); !errors.Is(err, domain.ErrConnectionNotFound) {
			t.Errorf("discover %q: %v", email, err)
		}
	}
	off := false
	_, _ = s.Update(ctx, "boss", "c-1", application.Change{Enabled: &off})
	if _, err := s.Discover(ctx, "zara@acme.uz"); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Fatalf("a disabled connection routes: %v", err)
	}
	if c, err = s.AddDomain(ctx, "boss", "c-1", "corp.uz", true); err != nil || !c.Serves("corp.uz") {
		t.Fatalf("trusted = %+v %v", c, err)
	}
	if c, err = s.RemoveDomain(ctx, "boss", "c-1", "acme.uz"); err != nil || len(c.Domains) != 1 {
		t.Fatalf("remove = %+v %v", c, err)
	}
	if _, err := s.AddDomain(ctx, "bob", "c-1", "x.uz", false); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("non-admin: %v", err)
	}
	if _, err := s.AddDomain(ctx, "boss", "c-404", "x.uz", false); !errors.Is(err, domain.ErrConnectionNotFound) {
		t.Errorf("unknown connection: %v", err)
	}
	if _, err := s.AddDomain(ctx, "boss", "c-1", "not a domain", false); !errors.Is(err, domain.ErrInvalidConnection) {
		t.Errorf("bad domain: %v", err)
	}
	if _, err := withDNSFailingSave(s).AddDomain(ctx, "boss", "c-1", "y.uz", false); err == nil {
		t.Error("save error swallowed")
	}
	if n := len(a.actions); n != 6 {
		t.Fatalf("audited %d changes: %v", n, a.actions)
	}
}

func withDNSFailingSave(*application.Service) *application.Service {
	repo := memory.New()
	s, _ := withDNS(repo, dns{})
	_, _ = s.Create(context.Background(), "boss", okta)
	broken, _ := withDNS(failing{repo}, dns{})
	return broken
}
