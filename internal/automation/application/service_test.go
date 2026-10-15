package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/automation/application"
	"github.com/bakhod1r/kyber/internal/automation/domain"
	"github.com/bakhod1r/kyber/internal/automation/infrastructure/memory"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
)

const (
	lead = "u-lead"
	dev  = "u-dev"
)

type projects struct{}

func (projects) Authorize(_ context.Context, actor, project string, admin bool) error {
	switch {
	case project != "KYB" || (actor != lead && actor != dev):
		return application.ErrProjectNotFound
	case admin && actor != lead:
		return application.ErrForbidden
	}
	return nil
}

// issues records what actions did; fail makes one action kind fail.
type issues struct {
	did  []string
	fail string
}

func (f *issues) do(s string) error {
	if f.fail != "" && strings.HasPrefix(s, f.fail) {
		return errors.New("not allowed")
	}
	f.did = append(f.did, s)
	return nil
}
func (f *issues) Transition(_ context.Context, actor, key, status string) error {
	return f.do("transition " + key + " " + status + " as " + actor)
}
func (f *issues) Assign(_ context.Context, actor, key, who string) error {
	return f.do("assign " + key + " " + who + " as " + actor)
}
func (f *issues) AddLabel(_ context.Context, actor, key, label string) error {
	return f.do("label " + key + " " + label + " as " + actor)
}
func (f *issues) Comment(_ context.Context, actor, key, body string) error {
	return f.do("comment " + key + " " + body + " as " + actor)
}

// conditions: "type = bug" matches KYB-1 only; "broken(" is invalid JQL.
type conditions struct{}

func (conditions) Validate(_ context.Context, _, project, jql string) error {
	if strings.Contains(jql, "broken(") {
		return fmt.Errorf("%w: unexpected (", application.ErrInvalidCondition)
	}
	return nil
}
func (conditions) Matches(_ context.Context, _, project, key, jql string) (bool, error) {
	return jql != "type = bug" || key == "KYB-1", nil
}

func setup() (*application.Service, *issues, *memory.Repository) {
	n := 0
	is, repo := &issues{}, memory.NewRepository()
	return application.NewService(application.Deps{Projects: projects{}, Issues: is, Conditions: conditions{}, Rules: repo,
		NewID: func() string { n++; return fmt.Sprintf("r-%d", n) }, Now: func() time.Time { return time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) }}), is, repo
}

func msg(name, actor, payload string) outbox.Message {
	return outbox.Message{Name: name, Actor: actor, Payload: []byte(payload)}
}

var reviewRule = domain.RuleSpec{Name: "Review", Trigger: domain.TriggerTransitioned, ToStatus: "in_review", Condition: "type = bug",
	Actions: []domain.Action{{Kind: domain.ActionAssign, Value: "reporter"}, {Kind: domain.ActionAddLabel, Value: "needs-review"}}}

func TestRulesAreForProjectAdmins(t *testing.T) {
	ctx := context.Background()
	s, _, _ := setup()
	if _, err := s.Create(ctx, dev, "KYB", reviewRule); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member creates: %v", err)
	}
	if _, err := s.Create(ctx, "u-eve", "KYB", reviewRule); !errors.Is(err, application.ErrProjectNotFound) {
		t.Fatalf("outsider: %v", err)
	}
	bad := reviewRule
	bad.Condition = "broken("
	if _, err := s.Create(ctx, lead, "KYB", bad); !errors.Is(err, application.ErrInvalidCondition) {
		t.Fatalf("invalid JQL: %v", err)
	}
	r, err := s.Create(ctx, lead, "KYB", reviewRule)
	if err != nil || r.Owner != lead {
		t.Fatalf("Create = %+v, %v", r, err)
	}
	if list, err := s.Rules(ctx, dev, "KYB"); err != nil || len(list) != 1 {
		t.Fatalf("members see rules: %v, %v", list, err)
	}
	off := false
	if _, err := s.Update(ctx, dev, string(r.ID), r.Version, reviewRule, &off); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member updates: %v", err)
	}
	if u, err := s.Update(ctx, lead, string(r.ID), r.Version, reviewRule, &off); err != nil || u.Enabled {
		t.Fatalf("Update = %+v, %v", u, err)
	}
	if _, err := s.Update(ctx, lead, string(r.ID), r.Version, reviewRule, &off); !errors.Is(err, domain.ErrRuleConflict) {
		t.Fatalf("stale update: %v", err)
	}
	if err := s.Delete(ctx, dev, string(r.ID)); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("member deletes: %v", err)
	}
	if err := s.Delete(ctx, lead, string(r.ID)); err != nil {
		t.Fatal(err)
	}
}

// A matching event runs the actions in order, as the rule's owner; the run is recorded.
func TestRuleRunsOnMatchingEvent(t *testing.T) {
	ctx := context.Background()
	s, is, _ := setup()
	_, _ = s.Create(ctx, lead, "KYB", reviewRule)
	if err := s.OnEvent(ctx, msg("issue.transitioned", dev, `{"key":"KYB-1","from":"in_progress","to":"in_review"}`)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(is.did, "; ") != "assign KYB-1 reporter as u-lead; label KYB-1 needs-review as u-lead" {
		t.Fatalf("did = %v", is.did)
	}
	runs, _ := s.Runs(ctx, dev, "KYB")
	if len(runs) != 1 || !runs[0].OK || runs[0].IssueKey != "KYB-1" {
		t.Fatalf("runs = %+v", runs)
	}
	// Other status, failing condition, or a change made by automation itself: nothing happens.
	for _, m := range []outbox.Message{
		msg("issue.transitioned", dev, `{"key":"KYB-1","to":"done"}`),
		msg("issue.transitioned", dev, `{"key":"KYB-2","to":"in_review"}`),
		msg("issue.transitioned", "", `{"key":"KYB-1","to":"in_review"}`),
	} {
		_ = s.OnEvent(ctx, m)
	}
	if len(is.did) != 2 {
		t.Fatalf("unexpected actions: %v", is.did)
	}
}

// A failing action stops the rule; the failure is recorded, and the event is not retried forever.
func TestFailingActionIsRecorded(t *testing.T) {
	ctx := context.Background()
	s, is, _ := setup()
	is.fail = "assign"
	_, _ = s.Create(ctx, lead, "KYB", reviewRule)
	if err := s.OnEvent(ctx, msg("issue.transitioned", dev, `{"key":"KYB-1","to":"in_review"}`)); err != nil {
		t.Fatalf("OnEvent = %v", err)
	}
	if len(is.did) != 0 {
		t.Fatalf("later actions ran after a failure: %v", is.did)
	}
	runs, _ := s.Runs(ctx, lead, "KYB")
	if len(runs) != 1 || runs[0].OK || !strings.Contains(runs[0].Detail, "not allowed") {
		t.Fatalf("runs = %+v", runs)
	}
}
