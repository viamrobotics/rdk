//go:build no_cgo

package nloptik

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// no_cgo builds leave the ik registry empty. Importers that need gradient-descent IK
// will get a clear error from ik.NewGradDescentSolver at call time.

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
func (s *NloptIK) Solve(ctx context.Context,
	solutionChan chan<- *ik.Solution,
	totalAttempts *atomic.Int32,
	seeds [][]float64,
	limits [][]referenceframe.Limit,
	minFunc ik.CostFunc,
	rseed int,
) (int, []ik.SeedSolveMetaData, error) {
	return 0, nil, errors.New("Cannot solve without cgo")
}

// DoF returns nil. The solver isn't real.
func (s *NloptIK) DoF() []referenceframe.Limit {
	return []referenceframe.Limit{}
}
