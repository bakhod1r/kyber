package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/access/domain"
)

const (
	lead    = domain.UserID("u-lead")
	dev     = domain.UserID("u-dev")
	viewer  = domain.UserID("u-viewer")
	qa      = domain.UserID("u-qa") // custom role "qa"
	alien   = domain.UserID("u-alien")
	auditor = domain.UserID("u-auditor") // granted by user, no role
)

func subject(u domain.UserID) domain.Subject {
	roles := map[domain.UserID]domain.RoleID{lead: domain.RoleAdmin, dev: domain.RoleMember, viewer: domain.RoleViewer, qa: "qa"}
	s := domain.Subject{User: u}
	if r, ok := roles[u]; ok {
		s.Roles = []domain.RoleID{r}
	}
	return s
}

func scheme(t *testing.T) *domain.Scheme {
	t.Helper()
	s := domain.DefaultScheme()
	s.Grant(domain.BrowseProjects, domain.Role("qa"))
	s.Grant(domain.TransitionIssues, domain.Role("qa"))
	s.Grant(domain.BrowseProjects, domain.User(auditor))
	s.Revoke(domain.CloseIssues, domain.Role(domain.RoleMember)) // this project: only the assignee (or an admin) closes
	s.Grant(domain.CloseIssues, domain.Assignee())
	s.Grant(domain.CloseIssues, domain.Role(domain.RoleAdmin))
	if err := s.AddLevel(domain.SecurityLevel{ID: "confidential", Name: "Confidential", Grants: []domain.Grant{domain.Role(domain.RoleAdmin), domain.Reporter()}}); err != nil {
		t.Fatal(err)
	}
	s.LockStatus("done")
	return s
}

func issue(reporter, assignee domain.UserID, status, level string) *domain.IssueAttrs {
	return &domain.IssueAttrs{Reporter: reporter, Assignee: assignee, Status: status, SecurityLevel: level}
}

func TestDecide(t *testing.T) {
	s := scheme(t)
	open := issue(dev, "", "todo", "")
	mine := issue(viewer, "", "todo", "")
	secret := issue(dev, "", "todo", "confidential")
	closed := issue(dev, dev, "done", "")
	assigned := issue(lead, dev, "in_progress", "")
	cases := []struct {
		name   string
		who    domain.UserID
		perm   domain.Permission
		on     *domain.IssueAttrs
		allow  bool
		reason string
	}{
		// RBAC: the default scheme reproduces viewer < member < admin.
		{"viewer browses", viewer, domain.BrowseProjects, nil, true, "role viewer"},
		{"viewer cannot create", viewer, domain.CreateIssues, nil, false, "no grant"},
		{"member creates", dev, domain.CreateIssues, nil, true, "role member"},
		{"member edits", dev, domain.EditIssues, open, true, "role member"},
		{"member cannot administer", dev, domain.AdministerProjects, nil, false, "no grant"},
		{"admin administers", lead, domain.AdministerProjects, nil, true, "role admin"},
		{"member manages sprints", dev, domain.ManageSprints, nil, true, "role member"},
		{"member cannot delete others' issues", dev, domain.DeleteIssues, assigned, false, "no grant"},
		{"member deletes own issue", dev, domain.DeleteIssues, open, true, "own issue (reporter)"},
		{"admin deletes", lead, domain.DeleteIssues, open, true, "role admin"},
		// Custom roles and user grants.
		{"custom role browses", qa, domain.BrowseProjects, nil, true, "role qa"},
		{"custom role transitions", qa, domain.TransitionIssues, open, true, "role qa"},
		{"custom role cannot edit", qa, domain.EditIssues, open, false, "no grant"},
		{"user grant browses", auditor, domain.BrowseProjects, nil, true, "user u-auditor"},
		{"non-member denied", alien, domain.BrowseProjects, nil, false, "cannot browse"},
		{"non-member denied anything", alien, domain.EditIssues, open, false, "cannot browse"},
		// ABAC: own permissions and attribute holders.
		{"viewer edits own issue", viewer, domain.EditIssues, mine, true, "own issue (reporter)"},
		{"viewer cannot edit others' issue", viewer, domain.EditIssues, open, false, "no grant"},
		{"viewer deletes own issue", viewer, domain.DeleteIssues, mine, true, "own issue (reporter)"},
		{"assignee closes", dev, domain.CloseIssues, assigned, true, "assignee"},
		{"non-assignee member cannot close", dev, domain.CloseIssues, open, false, "no grant"},
		{"admin closes anything", lead, domain.CloseIssues, open, true, "role admin"},
		{"assignee holder needs an issue", dev, domain.CloseIssues, nil, false, "no grant"},
		// ABAC: issue security levels.
		{"admin sees confidential", lead, domain.BrowseProjects, secret, true, "role admin"},
		{"reporter sees own confidential", dev, domain.BrowseProjects, secret, true, "role member"},
		{"viewer cannot see confidential", viewer, domain.BrowseProjects, secret, false, "security level confidential"},
		{"hidden issue cannot be edited either", qa, domain.TransitionIssues, secret, false, "security level confidential"},
		{"unknown level hides the issue", lead, domain.BrowseProjects, issue(dev, "", "todo", "ghost"), false, "unknown security level"},
		// ABAC: read-only statuses.
		{"closed issue is read-only", dev, domain.EditIssues, closed, false, "status done is read-only"},
		{"closed issue can still be reopened", dev, domain.TransitionIssues, closed, true, "role member"},
		{"closed issue still browsable", viewer, domain.BrowseProjects, closed, true, "role viewer"},
		{"comment on closed issue denied", dev, domain.AddComments, closed, false, "status done is read-only"},
		// Default deny.
		{"unknown permission", lead, domain.Permission("LAUNCH_ROCKETS"), nil, false, "unknown permission"},
		{"anonymous", "", domain.BrowseProjects, nil, false, "anonymous"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := s.Decide(subject(c.who), c.perm, c.on)
			if d.Allowed != c.allow || !strings.Contains(d.Reason, c.reason) {
				t.Fatalf("Decide(%s, %s) = %+v, want allowed=%v reason~%q", c.who, c.perm, d, c.allow, c.reason)
			}
		})
	}
}

func TestDecisionErr(t *testing.T) {
	s := scheme(t)
	if err := s.Decide(subject(dev), domain.EditIssues, issue(dev, "", "todo", "")).Err(); err != nil {
		t.Fatalf("allowed err = %v", err)
	}
	// Non-browsers get ErrNotFound (no existence leak); browsers lacking the permission ErrForbidden.
	if err := s.Decide(subject(alien), domain.EditIssues, nil).Err(); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("alien err = %v", err)
	}
	if err := s.Decide(subject(viewer), domain.BrowseProjects, issue(dev, "", "todo", "confidential")).Err(); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("hidden issue err = %v", err)
	}
	err := s.Decide(subject(viewer), domain.CreateIssues, nil).Err()
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "CREATE_ISSUES") {
		t.Fatalf("viewer err = %v", err)
	}
}

func TestSchemeValidation(t *testing.T) {
	s := domain.DefaultScheme()
	if err := s.AddLevel(domain.SecurityLevel{ID: "", Name: "x"}); !errors.Is(err, domain.ErrInvalidLevel) {
		t.Fatalf("empty id err = %v", err)
	}
	if err := s.AddLevel(domain.SecurityLevel{ID: "l", Name: "L", Grants: []domain.Grant{domain.Reporter()}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLevel(domain.SecurityLevel{ID: "l", Name: "again"}); !errors.Is(err, domain.ErrInvalidLevel) {
		t.Fatalf("duplicate err = %v", err)
	}
	// Granting twice is idempotent; revoking removes it.
	s.Grant(domain.DeleteIssues, domain.Role(domain.RoleMember))
	s.Grant(domain.DeleteIssues, domain.Role(domain.RoleMember))
	if n := len(s.Grants(domain.DeleteIssues)); n != 3 { // admin, own (reporter-only), member
		t.Fatalf("grants = %v", s.Grants(domain.DeleteIssues))
	}
	s.Revoke(domain.DeleteIssues, domain.Role(domain.RoleMember))
	if d := s.Decide(subject(dev), domain.DeleteIssues, issue(lead, "", "todo", "")); d.Allowed {
		t.Fatalf("revoked still allowed: %+v", d)
	}
	if got := domain.Permissions(); len(got) < 12 || got[0] != domain.BrowseProjects {
		t.Fatalf("permissions = %v", got)
	}
	for _, g := range []domain.Grant{domain.Role("qa"), domain.User("u-1"), domain.Reporter(), domain.Assignee()} {
		if g.String() == "" {
			t.Fatalf("grant %#v has no label", g)
		}
	}
}
