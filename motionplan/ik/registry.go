package ik

import (
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// SolverFactory builds a gradient-descent IK Solver. Parameters mirror the
// historical CreateNloptSolver signature; future solver implementations are
// expected to interpret them analogously.
type SolverFactory func(
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
	maxTime time.Duration,
) (Solver, error)

var defaultGradDescentFactory SolverFactory

// RegisterDefaultSolver wires a SolverFactory as the default gradient-descent
// solver returned by NewGradDescentSolver. Intended to be called from a
// package init() (see motionplan/ik/nloptik).
func RegisterDefaultSolver(f SolverFactory) {
	defaultGradDescentFactory = f
}

// NewGradDescentSolver returns a Solver built from the registered default
// factory. Returns an error if no factory has been registered — typically
// fixed by blank-importing an implementation package such as
// `_ "go.viam.com/rdk/motionplan/ik/nloptik"` in the binary or test entrypoint.
func NewGradDescentSolver(
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
	maxTime time.Duration,
) (Solver, error) {
	if defaultGradDescentFactory == nil {
		return nil, errors.New(
			`no gradient-descent IK solver registered; ` +
				`add: import _ "go.viam.com/rdk/motionplan/ik/nloptik"`)
	}
	return defaultGradDescentFactory(logger, iter, exact, useRelTol, maxTime)
}
