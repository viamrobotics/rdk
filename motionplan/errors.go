package motionplan

import (
	"errors"
	"fmt"
)

// test comment

var (
	errIKSolve = errors.New("zero IK solutions produced, goal positions appears to be physically unreachable")

	errPlannerFailed = errors.New("motion planner failed to find path")

	errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")

	errIKConstraint = "all IK solutions failed constraints. Failures: "
)

func genIKConstraintErr(failures map[string]int, constraintFailCnt int) error {
	ikConstraintFailures := errIKConstraint
	for failName, count := range failures {
		ikConstraintFailures += fmt.Sprintf("{ %s: %.2f%% }, ", failName, 100*float64(count)/float64(constraintFailCnt))
	}
	return errors.New(ikConstraintFailures)
}
