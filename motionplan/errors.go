package motionplan

import (
	"errors"
	"fmt"
	"slices"
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
	for _, k := range keys {
		errMsg += fmt.Sprintf("{ %s: %.2f%% }, ", k, 100*float64(failures[k])/float64(constraintFailCnt))
	}
	return errors.New(errMsg)
}
