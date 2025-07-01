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
	RRTStar PlanningAlgorithm = "rrtstar"
	// TPSpace indicates that TPSpaceMotionPlanner should be used.
	TPSpace PlanningAlgorithm = "tpspace"
	// UnspecifiedAlgorithm indicates that the use of our motion planning will accept whatever defaults the package
	// provides. As of the creation of this comment, that algorithm will be cBiRRT.
	UnspecifiedAlgorithm PlanningAlgorithm = ""
)

// AlgorithmSettings is a polymorphic representation of motion planning algorithms and their parameters. The `Algorithm`
// should correlate with the available options (e.g. if `Algorithm` us CBiRRT, RRTStarOpts should be nil and CBirrtOpts should not).
type AlgorithmSettings struct {
	Algorithm   PlanningAlgorithm      `json:"algorithm"`
	CBirrtOpts  *cbirrtOptions         `json:"cbirrt_settings"`
	RRTStarOpts *rrtStarConnectOptions `json:"rrtstar_settings"`
}

type plannerConstructor func(
	referenceframe.FrameSystem,
	*rand.Rand,
	logging.Logger,
	*plannerOptions,
	*ConstraintHandler,
	*AlgorithmSettings,
) (motionPlanner, error)

func newPlannerConstructor(algo PlanningAlgorithm) plannerConstructor {
	switch algo {
	case CBiRRT:
		return newCBiRRTMotionPlanner
	case RRTStar:
		return newRRTStarConnectMotionPlanner
	case TPSpace:
		return newTPSpaceMotionPlanner
	case UnspecifiedAlgorithm:
		return newCBiRRTMotionPlanner
	default:
		return newCBiRRTMotionPlanner
	}
}

func newMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
	constraintHandler *ConstraintHandler,
) (motionPlanner, error) {
	return newPlannerConstructor(opt.PlanningAlgorithm())(
		fs, seed, logger, opt, constraintHandler, &opt.PlanningAlgorithmSettings)
}
