package armplanning

import (
	"errors"
	"fmt"
	"slices"
)

const cutoffPercent = 10.0

var (
	errIKSolve = errors.New("zero IK solutions produced, goal positions appears to be physically unreachable")

	errPlannerFailed = errors.New("motion planner failed to find path")

	errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")

	errIKConstraint = "all IK solutions failed constraints. Failures: "

	errInvalidConstraint = errors.New("invalid constraint input")
)

// NewAlgAndConstraintMismatchErr is returned when an incompatible planning_alg is specified and there are contraints.
func NewAlgAndConstraintMismatchErr(planAlg string) error {
	return fmt.Errorf("cannot specify a planning algorithm other than cbirrt with topo constraints. algorithm specified was %s", planAlg)
}

func newIKConstraintErr(failures map[string]int, constraintFailCnt int) error {
	// sort the map keys by the integer they map to
	keys := make([]string, 0, len(failures))
	for k := range failures {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return failures[b] - failures[a] // descending order
	})

	// build the error message
	errMsg := errIKConstraint
	// determine if first error exceeds cutoff
	hasErrorAboveCutoff := 100*float64(failures[keys[0]])/float64(constraintFailCnt) >= cutoffPercent
	for _, k := range keys {
		percentageCollision := 100 * float64(failures[k]) / float64(constraintFailCnt)
		// print all errors if none exceed cutoff, else only those above cutoff
		if !hasErrorAboveCutoff || percentageCollision >= cutoffPercent {
			errMsg += fmt.Sprintf("{ %s: %.2f%% }, ", k, percentageCollision)
		}
	}
	return errors.New(errMsg)
}
