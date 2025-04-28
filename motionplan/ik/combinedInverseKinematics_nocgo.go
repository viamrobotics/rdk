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
	return nil, errors.New("motion planning is not supported on this build")
}
