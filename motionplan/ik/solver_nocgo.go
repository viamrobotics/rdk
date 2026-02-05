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
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
	maxTime time.Duration,
) (*NloptIK, error) {
	return nil, errors.New("nlopt is not supported on this build")
}

// NloptIK mimics the type in the cgo compiled code.
type NloptIK struct{}

// Solve refuses to solve problems without cgo.
func (ik *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *Solution,
	seeds [][]float64,
	limits [][]referenceframe.Limit,
	minFunc CostFunc,
	rseed int,
) (int, []SeedSolveMetaData, error) {
	return 0, nil, errors.New("Cannot solve without cgo")
}

// DoF returns nil. The solver isn't real.
func (ik *NloptIK) DoF() []referenceframe.Limit {
	return []referenceframe.Limit{}
}
