package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnknownStatus        = errors.New("unknown status")
	ErrTransitionNotAllowed = errors.New("transition not allowed by workflow")
	ErrInvalidWorkflow      = errors.New("invalid workflow")
	ErrWorkflowNotFound     = errors.New("project has no configured workflow")
)

type StatusID string

// Category is Jira's status category: it tells reports, boards and sprints what a status means,
// whatever the project calls it.
type Category string

const (
	CategoryToDo       Category = "todo"
	CategoryInProgress Category = "in_progress"
	CategoryDone       Category = "done"
)

func (c Category) Valid() bool {
	return c == CategoryToDo || c == CategoryInProgress || c == CategoryDone
}

// Status is one step of a workflow.
type Status struct {
	ID       StatusID `json:"id"`
	Name     string   `json:"name"`
	Category Category `json:"category"`
}

// Transition allows moving an issue From one status To another.
type Transition struct {
	From StatusID `json:"from"`
	To   StatusID `json:"to"`
}

// WorkflowDef is a project's workflow as configured. Without transitions every status can be
// reached from every other one (Jira's "all statuses can transition to this one").
type WorkflowDef struct {
	Initial     StatusID     `json:"initial"`
	Statuses    []Status     `json:"statuses"`
	Transitions []Transition `json:"transitions"`
}

const maxStatusName = 60

var statusIDPattern = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)

// Workflow is the graph of statuses and the transitions allowed between them.
type Workflow struct {
	initial     StatusID
	statuses    []Status
	explicit    []Transition
	transitions map[StatusID]map[StatusID]struct{}
}

// DefineWorkflow validates a configured workflow: unique ids and names, valid categories, at least
// one Done status (so issues can be resolved) and transitions between known, distinct statuses.
func DefineWorkflow(def WorkflowDef) (*Workflow, error) {
	if len(def.Statuses) == 0 {
		return nil, fmt.Errorf("no statuses: %w", ErrInvalidWorkflow)
	}
	wf := &Workflow{initial: def.Initial, transitions: make(map[StatusID]map[StatusID]struct{}, len(def.Statuses))}
	names := make(map[string]struct{}, len(def.Statuses))
	hasDone := false
	for _, s := range def.Statuses {
		s.Name = strings.TrimSpace(s.Name)
		name := strings.ToLower(s.Name)
		switch {
		case !statusIDPattern.MatchString(string(s.ID)):
			return nil, fmt.Errorf("status id %q: %w", s.ID, ErrInvalidWorkflow)
		case s.Name == "" || utf8.RuneCountInString(s.Name) > maxStatusName:
			return nil, fmt.Errorf("status %q name: %w", s.ID, ErrInvalidWorkflow)
		case !s.Category.Valid():
			return nil, fmt.Errorf("status %q category %q: %w", s.ID, s.Category, ErrInvalidWorkflow)
		case wf.Has(s.ID):
			return nil, fmt.Errorf("duplicate status %q: %w", s.ID, ErrInvalidWorkflow)
		}
		if _, dup := names[name]; dup {
			return nil, fmt.Errorf("duplicate status name %q: %w", s.Name, ErrInvalidWorkflow)
		}
		names[name] = struct{}{}
		hasDone = hasDone || s.Category == CategoryDone
		wf.statuses = append(wf.statuses, s)
		wf.transitions[s.ID] = map[StatusID]struct{}{}
	}
	if !wf.Has(def.Initial) {
		return nil, fmt.Errorf("initial %q: %w", def.Initial, ErrUnknownStatus)
	}
	if !hasDone {
		return nil, fmt.Errorf("no status in category done: %w", ErrInvalidWorkflow)
	}
	for _, t := range def.Transitions {
		if !wf.Has(t.From) || !wf.Has(t.To) {
			return nil, fmt.Errorf("%q -> %q: %w", t.From, t.To, ErrUnknownStatus)
		}
		if t.From == t.To {
			return nil, fmt.Errorf("%q -> itself: %w", t.From, ErrInvalidWorkflow)
		}
		wf.transitions[t.From][t.To] = struct{}{}
		wf.explicit = append(wf.explicit, t)
	}
	if len(def.Transitions) == 0 {
		for _, from := range wf.statuses {
			for _, to := range wf.statuses {
				if from.ID != to.ID {
					wf.transitions[from.ID][to.ID] = struct{}{}
				}
			}
		}
	}
	return wf, nil
}

func NewWorkflow(initial StatusID, transitions map[StatusID][]StatusID) (*Workflow, error) {
	wf := &Workflow{initial: initial, transitions: make(map[StatusID]map[StatusID]struct{}, len(transitions))}
	for from, tos := range transitions {
		set := make(map[StatusID]struct{}, len(tos))
		for _, to := range tos {
			set[to] = struct{}{}
		}
		wf.transitions[from] = set
	}
	if !wf.Has(initial) {
		return nil, fmt.Errorf("initial %q: %w", initial, ErrUnknownStatus)
	}
	for from, tos := range wf.transitions {
		for to := range tos {
			if !wf.Has(to) {
				return nil, fmt.Errorf("%q -> %q: %w", from, to, ErrUnknownStatus)
			}
		}
	}
	return wf, nil
}

func (w *Workflow) Initial() StatusID { return w.initial }

func (w *Workflow) Has(s StatusID) bool {
	_, ok := w.transitions[s]
	return ok
}

func (w *Workflow) CanTransition(from, to StatusID) bool {
	_, ok := w.transitions[from][to]
	return ok
}

// Statuses returns the statuses in their configured (board column) order.
func (w *Workflow) Statuses() []Status { return append([]Status(nil), w.statuses...) }

// Transitions returns the explicitly configured transitions; empty means any-to-any.
func (w *Workflow) Transitions() []Transition { return append([]Transition(nil), w.explicit...) }

// CategoryOf returns the status's category, or "" for an unknown status.
func (w *Workflow) CategoryOf(s StatusID) Category {
	for _, st := range w.statuses {
		if st.ID == s {
			return st.Category
		}
	}
	return ""
}

// firstIn returns the first status (in column order) of the category.
func (w *Workflow) firstIn(c Category) (StatusID, bool) {
	for _, st := range w.statuses {
		if st.Category == c {
			return st.ID, true
		}
	}
	return "", false
}

// IsDone reports whether the status resolves an issue.
func (w *Workflow) IsDone(s StatusID) bool { return w.CategoryOf(s) == CategoryDone }

const (
	StatusTodo       StatusID = "todo"
	StatusInProgress StatusID = "in_progress"
	StatusDone       StatusID = "done"
)

// DefaultWorkflowDef is Jira's simplified workflow: To Do, In Progress and Done, with every
// status reachable from every other one. Restrictive workflows are configured per project.
func DefaultWorkflowDef() WorkflowDef {
	return WorkflowDef{Initial: StatusTodo, Statuses: []Status{
		{ID: StatusTodo, Name: "To Do", Category: CategoryToDo},
		{ID: StatusInProgress, Name: "In Progress", Category: CategoryInProgress},
		{ID: StatusDone, Name: "Done", Category: CategoryDone},
	}}
}

func DefaultWorkflow() *Workflow {
	wf, err := DefineWorkflow(DefaultWorkflowDef())
	if err != nil {
		panic(err) // static definition; covered by tests
	}
	return wf
}
