//go:build no_cgo

package ik

import (
	"context"

	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"

	"go.viam.com/rdk/referenceframe"
)

// CreateNloptSolver is not supported on no_cgo builds.
func CreateNloptSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
) (*NloptIK, error) {
	return nil, errors.New("nlopt is not supported on this build")
}

// NloptIK mimics the type in the cgo compiled code.
type NloptIK struct{}

// Solve refuses to solve problems without cgo.
func (ik *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seeds [][]float64,
	travelPercent []float64,
	minFunc CostFunc,
	rseed int,
) (int, error) {
	return 0, errors.New("Cannot solve without cgo")
}

// DoF returns nil. The solver isn't real.
func (ik *NloptIK) DoF() []referenceframe.Limit {
	return []referenceframe.Limit{}
}
