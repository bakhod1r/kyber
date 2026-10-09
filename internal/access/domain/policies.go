package domain

import (
	"maps"
	"slices"
	"strings"

	guard "github.com/bakhod1r/guard/access/domain"
)

// Compiling a scheme into guard policies (ADR-0003): grants are allow policies; read-only
// statuses and security levels are deny policies that win by priority.
const (
	resource      = "issue"
	levelAction   = Permission("VIEW_SECURITY_LEVEL") // internal: may the subject see the issue's level?
	priorityDeny  = 10
	priorityAllow = 100
)

func action(p Permission) string { return strings.ToLower(string(p)) }

// request builds a guard request; issue attributes are absent for project-level checks.
func request(sub Subject, p Permission, is *IssueAttrs) guard.Request {
	roles := make([]any, len(sub.Roles))
	for i, r := range sub.Roles {
		roles[i] = string(r)
	}
	r := guard.Request{
		Subject:  guard.Subject{ID: string(sub.User), Attributes: map[string]any{"project_roles": roles}},
		Action:   action(p),
		Resource: guard.Resource{Type: resource, Attributes: map[string]any{}},
	}
	if is != nil {
		r.Resource.Attributes = map[string]any{"reporter": string(is.Reporter), "assignee": string(is.Assignee),
			"status": is.Status, "security_level": is.SecurityLevel}
	}
	return r
}

func (g Grant) condition() guard.Condition {
	switch g.kind {
	case holderRole:
		return guard.Condition{Field: "subject.project_roles", Operator: guard.OpContains, Value: []string{g.value}}
	case holderUser:
		return guard.Condition{Field: "subject.id", Operator: guard.OpEq, Value: []string{g.value}}
	case holderReporter:
		return guard.Condition{Field: "subject.id", Operator: guard.OpEq, Value: []string{"$resource.reporter"}}
	default:
		return guard.Condition{Field: "subject.id", Operator: guard.OpEq, Value: []string{"$resource.assignee"}}
	}
}

func policy(name string, p Permission, effect guard.Effect, priority int, root *guard.ConditionGroup) guard.Policy {
	return guard.Policy{Name: name, Resource: resource, Action: action(p), Effect: effect, Priority: priority, Enabled: true, Root: root}
}

func (s *Scheme) policies() []guard.Policy {
	var out []guard.Policy
	for _, p := range permissions {
		for _, g := range s.grants[p] {
			out = append(out, policy(g.String(), p, guard.Allow, priorityAllow, &guard.ConditionGroup{Conditions: []guard.Condition{g.condition()}}))
		}
	}
	for _, st := range s.locked {
		for _, p := range editing {
			out = append(out, policy("status "+st+" is read-only", p, guard.Deny, priorityDeny,
				&guard.ConditionGroup{Conditions: []guard.Condition{{Field: "resource.status", Operator: guard.OpEq, Value: []string{st}}}}))
		}
	}
	known := slices.Sorted(maps.Keys(s.levels))
	for _, id := range known {
		var holders []guard.Condition
		for _, g := range s.levels[id].Grants {
			holders = append(holders, g.condition())
		}
		out = append(out, policy("hidden by security level "+id, levelAction, guard.Deny, priorityDeny, &guard.ConditionGroup{
			Conditions: []guard.Condition{{Field: "resource.security_level", Operator: guard.OpEq, Value: []string{id}}},
			Groups:     []guard.ConditionGroup{{Operator: guard.Or, Negate: true, Conditions: holders}},
		}))
	}
	unknown := []guard.Condition{{Field: "resource.security_level", Operator: guard.OpNe, Value: []string{""}}}
	if len(known) > 0 {
		unknown = append(unknown, guard.Condition{Field: "resource.security_level", Operator: guard.OpNotIn, Value: known})
	}
	out = append(out,
		policy("unknown security level", levelAction, guard.Deny, priorityDeny, &guard.ConditionGroup{Conditions: unknown}),
		policy("security level visible", levelAction, guard.Allow, priorityAllow, nil))
	return out
}

// reason turns guard's wording into Kyber's.
func reason(d guard.Decision) string {
	r := d.Reason
	for _, prefix := range []string{"allowed by policy ", "denied by policy "} {
		r = strings.TrimPrefix(r, prefix)
	}
	if !d.Allowed && strings.HasPrefix(r, "no matching") {
		return "no grant"
	}
	return r
}
