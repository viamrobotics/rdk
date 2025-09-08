//go:build no_cgo

package armplanning

import (
	"math/rand"

	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

var errNotSupported = errors.New("not supported on this build")

func newCBiRRTMotionPlanner(
	fs *referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *motionplan.ConstraintChecker,
	motionChains *motionChains,
) (motionPlanner, error) {
	return nil, errNotSupported
}
