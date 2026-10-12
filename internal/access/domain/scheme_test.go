package domain_test

import (
	"errors"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/domain"
)

func TestParseGrantRoundTrip(t *testing.T) {
	for _, g := range []domain.Grant{domain.Role(domain.RoleViewer), domain.User("u-1"), domain.Reporter(), domain.Assignee()} {
		back, err := domain.ParseGrant(g.Kind(), g.Value())
		if err != nil || back != g {
			t.Errorf("%v → %v %v", g, back, err)
		}
	}
	for _, bad := range [][2]string{{"role", "owner"}, {"user", ""}, {"reporter", "x"}, {"assignee", "x"}, {"group", "devs"}} {
		if _, err := domain.ParseGrant(bad[0], bad[1]); !errors.Is(err, domain.ErrInvalidScheme) {
			t.Errorf("%v err = %v", bad, err)
		}
	}
}

func TestSchemeFrom(t *testing.T) {
	table := domain.DefaultScheme().Table()
	if len(table) != len(domain.Permissions()) || len(table[domain.ManageSprints]) != 2 {
		t.Fatalf("table = %v", table)
	}
	// Viewers may create issues in this project; members may not manage sprints.
	table[domain.CreateIssues] = append(table[domain.CreateIssues], domain.Role(domain.RoleViewer), domain.Role(domain.RoleViewer))
	table[domain.ManageSprints] = []domain.Grant{domain.Role(domain.RoleAdmin)}
	s, err := domain.SchemeFrom(table)
	if err != nil {
		t.Fatal(err)
	}
	vic := domain.Subject{User: "vic", Roles: []domain.RoleID{domain.RoleViewer}}
	dev := domain.Subject{User: "dev", Roles: []domain.RoleID{domain.RoleMember}}
	if !s.Decide(vic, domain.CreateIssues, nil).Allowed || s.Decide(dev, domain.ManageSprints, nil).Allowed {
		t.Fatal("custom grants not applied")
	}
	if n := len(s.Grants(domain.CreateIssues)); n != 3 {
		t.Fatalf("duplicate grant kept: %d", n)
	}

	for name, edit := range map[string]func(domain.GrantTable){
		"unknown permission":     func(t domain.GrantTable) { t["FLY"] = nil },
		"admins lose browse":     func(t domain.GrantTable) { t[domain.BrowseProjects] = []domain.Grant{domain.Role(domain.RoleMember)} },
		"admins lose administer": func(t domain.GrantTable) { t[domain.AdministerProjects] = []domain.Grant{domain.User("u-1")} },
	} {
		bad := domain.DefaultScheme().Table()
		edit(bad)
		if _, err := domain.SchemeFrom(bad); !errors.Is(err, domain.ErrInvalidScheme) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
