// Package agileacl is the anti-corruption layer from Issue Tracking to the Agile context.
package agileacl

import (
	"context"
	"errors"

	agileapp "github.com/bakhod1r/kyber/internal/agile/app"
	issueapp "github.com/bakhod1r/kyber/internal/issue/app"
)

type Agile interface {
	CanHold(ctx context.Context, project, sprint string) error
}

type Sprints struct{ agile Agile }

func New(a Agile) *Sprints { return &Sprints{agile: a} }

func (s *Sprints) CanHold(ctx context.Context, project, sprint string) error {
	if err := s.agile.CanHold(ctx, project, sprint); errors.Is(err, agileapp.ErrInvalidSprint) {
		return issueapp.ErrInvalidSprint
	} else {
		return err
	}
}
