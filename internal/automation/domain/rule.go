// Package domain is the Automation context: per-project rules — when something happens to an issue (trigger),
// and it matches a JQL condition, do something (actions) — Jira's automation rules, kept small.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidRule  = errors.New("invalid rule")
	ErrRuleNotFound = errors.New("rule not found")
	ErrRuleConflict = errors.New("rule was changed concurrently")
)

const (
	maxName      = 100
	maxCondition = 2000
	maxComment   = 2000
	maxActions   = 5
)

type RuleID string

// Trigger is the issue event a rule waits for.
type Trigger string

const (
	TriggerCreated      Trigger = "issue.created"
	TriggerTransitioned Trigger = "issue.transitioned"
	TriggerCommented    Trigger = "comment.added"
	TriggerAssigned     Trigger = "issue.assigned"
)

// Triggers are the outbox events rules listen to.
var Triggers = []Trigger{TriggerCreated, TriggerTransitioned, TriggerCommented, TriggerAssigned}

// ActionKind is what a rule does to the issue.
type ActionKind string

const (
	ActionTransition ActionKind = "transition" // Value: status id
	ActionAssign     ActionKind = "assign"     // Value: user id, "reporter" or "unassigned"
	ActionAddLabel   ActionKind = "add_label"  // Value: label
	ActionComment    ActionKind = "comment"    // Value: Markdown text
)

type Action struct {
	Kind  ActionKind `json:"kind"`
	Value string     `json:"value"`
}

// RuleSpec is what a project admin writes.
type RuleSpec struct {
	Name      string
	Trigger   Trigger
	ToStatus  string // transitioned only: the status reached ("" = any)
	Condition string // JQL the issue must match ("" = always)
	Actions   []Action
}

// Rule runs its actions as its owner, so it can do only what the owner may do.
type Rule struct {
	ID      RuleID
	Project string
	Owner   string
	RuleSpec
	Enabled bool
	Version int
}

func NewRule(id RuleID, project, owner string, spec RuleSpec) (*Rule, error) {
	r := &Rule{ID: id, Project: project, Owner: owner, Enabled: true}
	return r, r.Redefine(spec)
}

// Redefine replaces what the rule does; on error nothing changes.
func (r *Rule) Redefine(s RuleSpec) error {
	s.Name, s.ToStatus, s.Condition = strings.TrimSpace(s.Name), strings.TrimSpace(s.ToStatus), strings.TrimSpace(s.Condition)
	switch {
	case s.Name == "" || utf8.RuneCountInString(s.Name) > maxName:
		return fmt.Errorf("%w: name must be 1-%d characters", ErrInvalidRule, maxName)
	case !slices.Contains(Triggers, s.Trigger):
		return fmt.Errorf("%w: unknown trigger %q", ErrInvalidRule, s.Trigger)
	case s.ToStatus != "" && s.Trigger != TriggerTransitioned:
		return fmt.Errorf("%w: a target status needs the transition trigger", ErrInvalidRule)
	case utf8.RuneCountInString(s.Condition) > maxCondition:
		return fmt.Errorf("%w: condition must be at most %d characters", ErrInvalidRule, maxCondition)
	case len(s.Actions) == 0 || len(s.Actions) > maxActions:
		return fmt.Errorf("%w: a rule has 1-%d actions", ErrInvalidRule, maxActions)
	}
	actions := make([]Action, len(s.Actions))
	for i, a := range s.Actions {
		a.Value = strings.TrimSpace(a.Value)
		if err := a.check(); err != nil {
			return err
		}
		actions[i] = a
	}
	s.Actions = actions
	r.RuleSpec = s
	return nil
}

func (a Action) check() error {
	if a.Value == "" {
		return fmt.Errorf("%w: %s needs a value", ErrInvalidRule, a.Kind)
	}
	switch a.Kind {
	case ActionTransition, ActionAssign:
		return nil
	case ActionAddLabel:
		if strings.ContainsAny(a.Value, " \t\n") || utf8.RuneCountInString(a.Value) > 50 {
			return fmt.Errorf("%w: a label is one word of at most 50 characters", ErrInvalidRule)
		}
		return nil
	case ActionComment:
		if utf8.RuneCountInString(a.Value) > maxComment {
			return fmt.Errorf("%w: a comment is at most %d characters", ErrInvalidRule, maxComment)
		}
		return nil
	}
	return fmt.Errorf("%w: unknown action %q", ErrInvalidRule, a.Kind)
}

func (r *Rule) Enable()  { r.Enabled = true }
func (r *Rule) Disable() { r.Enabled = false }

// Event is an issue event as rules see it.
type Event struct {
	Trigger  Trigger
	Project  string
	IssueKey string
	ToStatus string // transitioned only
	Actor    string // the person whose change it was
}

// Matches tells whether the enabled rule reacts to the event (the condition is checked separately).
func (r *Rule) Matches(e Event) bool {
	return r.Enabled && e.Project == r.Project && e.Trigger == r.Trigger && (r.ToStatus == "" || r.ToStatus == e.ToStatus)
}

// EventFrom reads an outbox issue event. Events without an actor — made by automation itself or a system
// job — start nothing, so rules cannot trigger each other in a loop.
func EventFrom(name, actor string, payload []byte) (Event, bool) {
	if actor == "" || !slices.Contains(Triggers, Trigger(name)) {
		return Event{}, false
	}
	var p struct {
		Key      string `json:"key"`
		IssueKey string `json:"issue_key"`
		To       string `json:"to"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return Event{}, false
	}
	key := p.Key
	if key == "" {
		key = p.IssueKey
	}
	i := strings.LastIndexByte(key, '-')
	if i <= 0 {
		return Event{}, false
	}
	e := Event{Trigger: Trigger(name), Project: key[:i], IssueKey: key, Actor: actor}
	if e.Trigger == TriggerTransitioned {
		e.ToStatus = p.To
	}
	return e, true
}

// Run is one execution of a rule on an issue, for the rule's audit trail.
type Run struct {
	Rule     RuleID
	Project  string
	IssueKey string
	At       time.Time
	OK       bool
	Detail   string // what was done, or why it failed
}

// Repository stores rules and their recent runs in the workspace of ctx.
type Repository interface {
	// Save inserts (Version 0) or updates with optimistic locking, then bumps Version.
	Save(ctx context.Context, r *Rule) error
	ByID(ctx context.Context, id RuleID) (*Rule, error)
	// ForProject lists the project's rules by name.
	ForProject(ctx context.Context, project string) ([]*Rule, error)
	Delete(ctx context.Context, id RuleID) error
	// Record appends a run; Runs lists the project's newest runs first.
	Record(ctx context.Context, run Run) error
	Runs(ctx context.Context, project string, limit int) ([]Run, error)
}
