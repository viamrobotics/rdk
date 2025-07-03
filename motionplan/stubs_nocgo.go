//go:build no_cgo

package motionplan

import (
	"math/rand"

	"github.com/pkg/errors"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const pathdebug = false

var errNotSupported = errors.New("not supported on this build")
var flipPose = spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

func newCBiRRTMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	motionChains *motionChains,
) (motionPlanner, error) {
	return nil, errNotSupported
}

// newTPSpaceMotionPlanner creates a newTPSpaceMotionPlanner object with a user specified random seed.
func newTPSpaceMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	motionChains *motionChains,
) (motionPlanner, error) {
	return nil, errNotSupported
}

// NewRRTStarConnectMotionPlannerWithSeed creates a rrtStarConnectMotionPlanner object with a user specified random seed.
func newRRTStarConnectMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	motionChains *motionChains,
) (motionPlanner, error) {
	return nil, errNotSupported
}

func rectifyTPspacePath(path []node, frame referenceframe.Frame, startPose spatialmath.Pose) ([]node, error) {
	return nil, errNotSupported
}
