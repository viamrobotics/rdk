package motionplan

import (
	"math/rand"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
)

// PlanningAlgorithm is an enum that determines which implmentation of motion planning should be used.
// It is meant to be passed to getPlanner (along with the arguments expected by the plannerConstructor
// function type) in order to acquire an implementer of motionPlanner.
type PlanningAlgorithm int

const (
	// CBiRRT indicates that a CBiRRTMotionPlanner should be used. This is currently the
	// default motion planner.
	CBiRRT PlanningAlgorithm = iota
	// RRTStar indicates that an RRTStarConnectMotionPlanner should be used.
	RRTStar
	// TPSpace indicates that TPSpaceMotionPlanner should be used.
	TPSpace
)

type plannerConstructor func(referenceframe.FrameSystem, *rand.Rand, logging.Logger, *plannerOptions) (motionPlanner, error)

func getPlannerConstructor(algo PlanningAlgorithm) plannerConstructor {
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

func getPlanner(
	algo PlanningAlgorithm,
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	return getPlannerConstructor(algo)(fs, seed, logger, opt)
}
