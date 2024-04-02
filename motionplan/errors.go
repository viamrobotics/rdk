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

	errBadPlanImpl = errors.New("rrtPlan is the only supported implementation of Plan by this function")
)

func genIKConstraintErr(failures map[string]int, constraintFailCnt int) error {
	ikConstraintFailures := errIKConstraint
	for failName, count := range failures {
		ikConstraintFailures += fmt.Sprintf("{ %s: %.2f%% }, ", failName, 100*float64(count)/float64(constraintFailCnt))
	}
	return errors.New(ikConstraintFailures)
}
