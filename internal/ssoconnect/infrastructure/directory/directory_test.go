package directory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identity "github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/oidcprovider"
	"github.com/bakhod1r/kyber/internal/ssoconnect/domain"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/directory"
	"github.com/bakhod1r/kyber/internal/ssoconnect/infrastructure/memory"
)

type fake struct{ issuer string }

func (fake) AuthURL(context.Context, identity.OIDCFlow) (string, error) { return "", nil }
func (fake) Exchange(context.Context, string, identity.OIDCFlow) (identity.ExternalProfile, error) {
	return identity.ExternalProfile{}, nil
}

type broken struct{ domain.Connections }

func (broken) All(context.Context) ([]domain.Connection, error) { return nil, errors.New("db down") }
func (broken) ByID(context.Context, domain.ConnectionID) (*domain.Connection, error) {
	return nil, errors.New("db down")
}

func add(t *testing.T, repo domain.Connections, id, name string, enabled bool) *domain.Connection {
	t.Helper()
	c, _ := domain.NewConnection(domain.ConnectionID(id), domain.Settings{Name: name, Issuer: "https://" + id + ".example", ClientID: "c", ClientSecret: "s"}, time.Now())
	if !enabled {
		c.Disable()
	}
	if err := repo.Save(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDirectory(t *testing.T) {
	ctx := context.Background()
	repo := memory.New()
	builds := 0
	d := directory.New(repo, func(c domain.Connection) identity.RedirectProvider { builds++; return fake{c.Issuer} })
	a := add(t, repo, "a", "Okta", true)
	add(t, repo, "b", "Old", false)
	list, err := d.SSOConnections(ctx)
	if err != nil || len(list) != 1 || list[0].ID != "a" || list[0].Name != "Okta" {
		t.Fatalf("list = %+v %v", list, err)
	}
	if c, err := d.SSOConnection(ctx, "a"); err != nil || c.Provider.(fake).issuer != "https://a.example" || builds != 1 {
		t.Fatalf("lookup = %+v %v (builds %d)", c, err, builds)
	}
	_ = a.Rename("Okta EU")
	_ = repo.Save(ctx, a)
	if c, _ := d.SSOConnection(ctx, "a"); c.Name != "Okta EU" || builds != 2 {
		t.Fatalf("a changed connection is rebuilt: %+v (builds %d)", c, builds)
	}
	for _, id := range []string{"b", "missing"} {
		if _, err := d.SSOConnection(ctx, id); !errors.Is(err, identity.ErrSSOConnectionNotFound) {
			t.Errorf("%s: %v", id, err)
		}
	}
	bad := directory.New(broken{repo}, nil)
	if _, err := bad.SSOConnections(ctx); err == nil {
		t.Error("list error swallowed")
	}
	if _, err := bad.SSOConnection(ctx, "a"); err == nil || errors.Is(err, identity.ErrSSOConnectionNotFound) {
		t.Errorf("lookup error: %v", err)
	}
}

func TestOIDCBuild(t *testing.T) {
	c, _ := domain.NewConnection("c-1", domain.Settings{Name: "X", Issuer: "https://x.example", ClientID: "id", ClientSecret: "s"}, time.Now())
	var got string
	p := directory.OIDC(func(id string) string { got = id; return "https://kyber.example/api/v1/auth/sso/" + id + "/callback" })(*c)
	if _, ok := p.(*oidcprovider.Client); !ok || got != "c-1" {
		t.Fatalf("build = %T (callback for %q)", p, got)
	}
}
