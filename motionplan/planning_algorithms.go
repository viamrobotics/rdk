package motionplan

import (
	"math/rand"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// PlanningAlgorithm is an enum that determines which implmentation of motion planning should be used.
// It is meant to be passed to newMotionPlanner (along with the arguments expected by the plannerConstructor
// function type) in order to acquire an implementer of motionPlanner.
type PlanningAlgorithm string

const (
	// CBiRRT indicates that a CBiRRTMotionPlanner should be used. This is currently the
	// default motion planner.
	CBiRRT PlanningAlgorithm = "cbirrt"
	// RRTStar indicates that an RRTStarConnectMotionPlanner should be used.
	RRTStar = "rrtstar"
	// TPSpace indicates that TPSpaceMotionPlanner should be used.
	TPSpace = "tpspace"
)

type plannerConstructor func(
	referenceframe.FrameSystem,
	*rand.Rand,
	logging.Logger,
	*plannerOptions,
	*ConstraintHandler,
) (motionPlanner, error)

func newPlannerConstructor(algo PlanningAlgorithm) plannerConstructor {
	switch algo {
	case CBiRRT:
		return newCBiRRTMotionPlanner
	case RRTStar:
		return newRRTStarConnectMotionPlanner
	case TPSpace:
		return newTPSpaceMotionPlanner
	default:
		return newCBiRRTMotionPlanner
	}
}

func newMotionPlanner(
	algo PlanningAlgorithm,
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
	constraintHandler *ConstraintHandler,
) (motionPlanner, error) {
	return newPlannerConstructor(algo)(fs, seed, logger, opt, constraintHandler)
}
