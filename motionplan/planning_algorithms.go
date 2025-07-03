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

// move back to cBiRRT.go when motionplan is taken out of RDK.
type cbirrtOptions struct {
	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	SolutionsToSeed int `json:"solutions_to_seed"`

	// This is how far cbirrt will try to extend the map towards a goal per-step. Determined from FrameStep
	qstep map[string][]float64
}

// move back to rrtStarConnect.go when motionplan is taken out of RDK.
type rrtStarConnectOptions struct {
	// The number of nearest neighbors to consider when adding a new sample to the tree
	NeighborhoodSize int `json:"neighborhood_size"`

	// This is how far rrtStarConnect will try to extend the map towards a goal per-step
	qstep map[string][]float64
}

type plannerConstructor func(
	referenceframe.FrameSystem,
	*rand.Rand,
	logging.Logger,
	*PlannerOptions,
	*ConstraintHandler,
	*motionChains,
) (motionPlanner, error)

func newMotionPlanner(
	fs referenceframe.FrameSystem,
	seed *rand.Rand,
	logger logging.Logger,
	opt *PlannerOptions,
	constraintHandler *ConstraintHandler,
	chains *motionChains,
) (motionPlanner, error) {
	switch opt.PlanningAlgorithm() {
	case CBiRRT:
		return newCBiRRTMotionPlanner(fs, seed, logger, opt, constraintHandler, chains)
	case RRTStar:
		return newRRTStarConnectMotionPlanner(fs, seed, logger, opt, constraintHandler, chains)
	case TPSpace:
		return newTPSpaceMotionPlanner(fs, seed, logger, opt, constraintHandler, chains)
	case UnspecifiedAlgorithm:
		fallthrough
	default:
		return newCBiRRTMotionPlanner(fs, seed, logger, opt, constraintHandler, chains)
	}
}
