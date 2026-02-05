package armplanning

import (
	"errors"
	"fmt"
	"slices"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

const cutoffPercent = 10.0

var (
	errIKSolve = errors.New("zero IK solutions produced, goal positions appears to be physically unreachable")

	errPlannerFailed = errors.New("motion planner failed to find path")

	errNoPlannerOptions = errors.New("PlannerOptions are required but have not been specified")

	errIKConstraint = "all IK solutions failed constraints. Failures: "
)

// NewAlgAndConstraintMismatchErr is returned when an incompatible planning_alg is specified and there are contraints.
func NewAlgAndConstraintMismatchErr(planAlg string) error {
	return fmt.Errorf("cannot specify a planning algorithm other than cbirrt with topo constraints. algorithm specified was %s", planAlg)
}

// IkConstraintError contains information on possible solutions that fail constraint checks. This
// data can be used to visualize the constraint thats being violated.
type IkConstraintError struct {
	// A map keeping track of which constraints fail
	FailuresByType map[string][]*referenceframe.LinearInputs
	// Count is the total number of failures. Equivalent to summing the size of the value slices in
	// `FailuresByType`.
	Count int

	Fs      *referenceframe.FrameSystem
	Checker *motionplan.ConstraintChecker
}

func newIkConstraintError(fs *referenceframe.FrameSystem, checker *motionplan.ConstraintChecker) *IkConstraintError {
	return &IkConstraintError{
		FailuresByType: make(map[string][]*referenceframe.LinearInputs),
		Fs:             fs,
		Checker:        checker,
	}
}

func (fail *IkConstraintError) add(solution *referenceframe.LinearInputs, err error) {
	fail.FailuresByType[err.Error()] = append(fail.FailuresByType[err.Error()], solution)
	fail.Count++
}

// OutputString formats the structure as a string. If pretty is true, there will be newlines and
// indentation for prettier formatting.
func (fail *IkConstraintError) OutputString(pretty bool) string {
	// sort the map keys by the integer they map to
	keys := make([]string, 0, len(fail.FailuresByType))
	for k := range fail.FailuresByType {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return len(fail.FailuresByType[b]) - len(fail.FailuresByType[a]) // descending order
	})

	// build the error message
	errMsg := errIKConstraint
	// determine if first error exceeds cutoff
	hasErrorAboveCutoff := 100*float64(len(fail.FailuresByType[keys[0]]))/float64(fail.Count) >= cutoffPercent
	for _, k := range keys {
		percentageCollision := 100 * float64(len(fail.FailuresByType[k])) / float64(fail.Count)
		// print all errors if none exceed cutoff, else only those above cutoff
		if !hasErrorAboveCutoff || percentageCollision >= cutoffPercent {
			if pretty {
				errMsg += fmt.Sprintf("\n  %.2f%% %s", percentageCollision, k)
			} else {
				errMsg += fmt.Sprintf("{ %s: %.2f%% }, ", k, percentageCollision)
			}
		}
	}

	return errMsg
}

func (fail *IkConstraintError) Error() string {
	return fail.OutputString(false)
}
