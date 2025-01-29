package motionplan

import (
	"errors"
	"fmt"
)

var (
	errIKSolve = errors.New("zero IK solutions produced, goal positions appears to be physically unreachable")

	errPlannerFailed = errors.New("motion planner failed to find path")

	errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")

	errIKConstraint = "all IK solutions failed constraints. Failures: "

	errNoNeighbors = errors.New("no neighbors found")

	errInvalidCandidate = errors.New("candidate did not meet constraints")

	errNoCandidates = errors.New("no candidates passed in, skipping")

	errInvalidConstraint = errors.New("invalid constraint input")

	errHighReplanCost = errors.New("unable to create a new plan within replanCostFactor from the original")

	// TODO: This should eventually be possible.
	errMixedFrameTypes = errors.New("unable to plan for PTG and non-PTG frames simultaneously")
)

// NewAlgAndConstraintMismatchErr is returned when an incompatible planning_alg is specified and there are contraints.
func NewAlgAndConstraintMismatchErr(planAlg string) error {
	return fmt.Errorf("cannot specify a planning alg other than cbirrt with topo constraints. alg specified was %s", planAlg)
}

func newIKConstraintErr(failures map[string]int, constraintFailCnt int) error {
	ikConstraintFailures := errIKConstraint
	for failName, count := range failures {
		ikConstraintFailures += fmt.Sprintf("{ %s: %.2f%% }, ", failName, 100*float64(count)/float64(constraintFailCnt))
	}
	return errors.New(ikConstraintFailures)
}
