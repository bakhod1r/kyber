package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/automation/domain"
)

func valid() domain.RuleSpec {
	return domain.RuleSpec{Name: "Close on merge", Trigger: domain.TriggerTransitioned, ToStatus: "in_review",
		Actions: []domain.Action{{Kind: domain.ActionAssign, Value: "reporter"}, {Kind: domain.ActionComment, Value: "Ready for review"}}}
}

func TestNewRule(t *testing.T) {
	r, err := domain.NewRule("r-1", "KYB", "u-lead", valid())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Enabled || r.Owner != "u-lead" || r.Project != "KYB" || len(r.Actions) != 2 {
		t.Fatalf("rule = %+v", r)
	}
	bad := map[string]func(*domain.RuleSpec){
		"no name":          func(s *domain.RuleSpec) { s.Name = " " },
		"long name":        func(s *domain.RuleSpec) { s.Name = strings.Repeat("x", 101) },
		"unknown trigger":  func(s *domain.RuleSpec) { s.Trigger = "issue.deleted" },
		"no actions":       func(s *domain.RuleSpec) { s.Actions = nil },
		"too many actions": func(s *domain.RuleSpec) { s.Actions = make([]domain.Action, 6) },
		"unknown action":   func(s *domain.RuleSpec) { s.Actions = []domain.Action{{Kind: "delete", Value: "x"}} },
		"empty value":      func(s *domain.RuleSpec) { s.Actions = []domain.Action{{Kind: domain.ActionAddLabel, Value: " "}} },
		"label with space": func(s *domain.RuleSpec) { s.Actions = []domain.Action{{Kind: domain.ActionAddLabel, Value: "a b"}} },
		"long comment": func(s *domain.RuleSpec) {
			s.Actions = []domain.Action{{Kind: domain.ActionComment, Value: strings.Repeat("x", 2001)}}
		},
		"status off trigger": func(s *domain.RuleSpec) { s.Trigger, s.ToStatus = domain.TriggerCreated, "done" },
	}
	for name, mutate := range bad {
		s := valid()
		mutate(&s)
		if _, err := domain.NewRule("r-2", "KYB", "u", s); !errors.Is(err, domain.ErrInvalidRule) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRuleMatches(t *testing.T) {
	r, _ := domain.NewRule("r-1", "KYB", "u-lead", valid())
	cases := []struct {
		name string
		ev   domain.Event
		want bool
	}{
		{"transition to the status", domain.Event{Trigger: domain.TriggerTransitioned, Project: "KYB", IssueKey: "KYB-1", ToStatus: "in_review"}, true},
		{"transition elsewhere", domain.Event{Trigger: domain.TriggerTransitioned, Project: "KYB", IssueKey: "KYB-1", ToStatus: "done"}, false},
		{"other trigger", domain.Event{Trigger: domain.TriggerCreated, Project: "KYB", IssueKey: "KYB-1"}, false},
		{"other project", domain.Event{Trigger: domain.TriggerTransitioned, Project: "OPS", IssueKey: "OPS-1", ToStatus: "in_review"}, false},
	}
	for _, c := range cases {
		if got := r.Matches(c.ev); got != c.want {
			t.Errorf("%s: Matches = %v", c.name, got)
		}
	}
	any := valid()
	any.ToStatus = ""
	r2, _ := domain.NewRule("r-2", "KYB", "u", any)
	if !r2.Matches(domain.Event{Trigger: domain.TriggerTransitioned, Project: "KYB", IssueKey: "KYB-1", ToStatus: "done"}) {
		t.Fatal("no status = any transition")
	}
	r.Disable()
	if r.Matches(cases[0].ev) {
		t.Fatal("a disabled rule never matches")
	}
}

// EventFrom reads the issue events rules react to; events without a person behind them (automation's own
// actions, system jobs) start nothing, so rules cannot loop.
func TestEventFrom(t *testing.T) {
	ev, ok := domain.EventFrom("issue.transitioned", "u-dev", []byte(`{"key":"KYB-7","from":"todo","to":"done"}`))
	if !ok || ev.Trigger != domain.TriggerTransitioned || ev.Project != "KYB" || ev.IssueKey != "KYB-7" || ev.ToStatus != "done" || ev.Actor != "u-dev" {
		t.Fatalf("event = %+v %v", ev, ok)
	}
	if ev, ok := domain.EventFrom("comment.added", "u-dev", []byte(`{"issue_key":"KYB-7","body":"hi"}`)); !ok || ev.IssueKey != "KYB-7" {
		t.Fatalf("comment = %+v %v", ev, ok)
	}
	if ev, ok := domain.EventFrom("issue.created", "u-dev", []byte(`{"key":"KYB-8"}`)); !ok || ev.Trigger != domain.TriggerCreated {
		t.Fatalf("created = %+v %v", ev, ok)
	}
	if _, ok := domain.EventFrom("issue.transitioned", "", []byte(`{"key":"KYB-7","to":"done"}`)); ok {
		t.Fatal("a change made by automation starts nothing")
	}
	if _, ok := domain.EventFrom("issue.created", "u", []byte(`garbage`)); ok {
		t.Fatal("malformed")
	}
	if _, ok := domain.EventFrom("sprint.started", "u", []byte(`{}`)); ok {
		t.Fatal("unknown event")
	}
}
