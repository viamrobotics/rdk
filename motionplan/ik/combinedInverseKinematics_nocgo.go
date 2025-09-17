//go:build no_cgo

package ik

import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"

	"go.viam.com/rdk/referenceframe"
)

// CreateCombinedIKSolver is not supported on no_cgo builds.
func CreateCombinedIKSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	nCPU int,
	goalThreshold float64,
) (Solver, error) {
	return nil, errors.New("nlopt is not supported on this build")
}

// CreateNloptSolver is not supported on no_cgo builds.
func CreateNloptSolver(
	limits []referenceframe.Limit,
	logger logging.Logger,
	iter int,
	exact, useRelTol bool,
) (Solver, error) {
	return nil, errors.New("nlopt is not supported on this build")
}
