package domain

import "testing"

// Compiled policies must be valid guard policies, so guard's stores and admin UI accept them.
func TestCompiledPoliciesAreValidGuardPolicies(t *testing.T) {
	s := DefaultScheme()
	s.LockStatus("done")
	_ = s.AddLevel(SecurityLevel{ID: "confidential", Grants: []Grant{Role(RoleAdmin), Reporter(), Assignee(), User("u-1")}})
	ps := s.policies()
	if len(ps) < 20 {
		t.Fatalf("policies = %d", len(ps))
	}
	for i := range ps {
		if err := ps[i].Validate(); err != nil {
			t.Errorf("%s/%s %q: %v", ps[i].Resource, ps[i].Action, ps[i].Name, err)
		}
	}
}
