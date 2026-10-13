package domain

import (
	"errors"
	"fmt"
	"slices"
)

// Resolution is how a resolved issue ended (Jira's resolution field); "" = unresolved.
type Resolution string

const (
	ResolutionDone            Resolution = "done"
	ResolutionWontDo          Resolution = "wont_do"
	ResolutionDuplicate       Resolution = "duplicate"
	ResolutionCannotReproduce Resolution = "cannot_reproduce"
)

var ErrInvalidResolution = errors.New("invalid resolution: an issue is resolved only in a Done status")

var resolutions = []Resolution{ResolutionDone, ResolutionWontDo, ResolutionDuplicate, ResolutionCannotReproduce}

// Resolutions lists every resolution in display order.
func Resolutions() []Resolution { return slices.Clone(resolutions) }

func ParseResolution(s string) (Resolution, error) {
	if r := Resolution(s); slices.Contains(resolutions, r) {
		return r, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidResolution, s)
}

// Resolution is "" while the issue is not in a Done status.
func (i *Issue) Resolution() Resolution { return i.resolution }

// ChangeResolution changes how a resolved issue ended; an unresolved issue gets one only by a
// transition into a Done status.
func (i *Issue) ChangeResolution(res Resolution) error {
	if i.resolution == "" {
		return ErrInvalidResolution
	}
	if _, err := ParseResolution(string(res)); err != nil {
		return err
	}
	i.resolution = res
	return nil
}

// TransitionResolved moves the issue like Transition. Entering a Done status resolves it as res
// (ResolutionDone when ""); leaving Done clears the resolution. res outside Done is invalid.
func (i *Issue) TransitionResolved(to StatusID, wf *Workflow, res Resolution) error {
	done := wf.IsDone(to)
	if res != "" && !done {
		return ErrInvalidResolution
	}
	if err := i.Transition(to, wf); err != nil { // sets ResolutionDone on entering Done
		return err
	}
	if res != "" {
		i.resolution = res
	}
	return nil
}
