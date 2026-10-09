package domain

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownStatus        = errors.New("unknown status")
	ErrTransitionNotAllowed = errors.New("transition not allowed by workflow")
)

type StatusID string

// Workflow is the graph of statuses and the transitions allowed between them.
type Workflow struct {
	initial     StatusID
	transitions map[StatusID]map[StatusID]struct{}
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
