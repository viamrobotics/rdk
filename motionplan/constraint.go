package motionplan

import (
	"go.viam.com/core/kinematics"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

type constraintInput struct {
	startPos   spatial.Pose
	endPos     spatial.Pose
	startInput []frame.Input
	endInput   []frame.Input
	frame      frame.Frame
}

var checkSeq = []float64{0.5, 0.333, 0.25, 0.17}

// constraintHandler is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse
type constraintHandler struct {
	constraints map[string]func(constraintInput) bool
}

// AddConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *constraintHandler) AddConstraint(name string, cons func(constraintInput) bool) {
	c.constraints[name] = cons
}

// RemoveConstraint will remove the given constraint
func (c *constraintHandler) RemoveConstraint(name string) {
	delete(c.constraints, name)
}

// Constraints will list all constraints by name
func (c *constraintHandler) Constraints() []string {
	names := make([]string, 0, len(c.constraints))
	for name := range c.constraints {
		names = append(names, name)
	}
	return names
}

// Create a constraint function from an arbitrary function that will decide if a given pose is valid.
// This function will check the given function at each point in checkSeq, and 1-point. If all constraints are satisfied,
// it will return true. If any intermediate pose violates the constraint, will return false. 
func NewPoseConstraint(oFunc func(spatial.Pose) bool) func(constraintInput) bool {
	return func(cInput constraintInput) bool{
		for _, s := range checkSeq {
			iPos, err := cInput.frame.Transform(kinematics.InterpolateValues(cInput.startInput, cInput.endInput, s))
			if !oFunc(iPos) || err != nil {
				return false
			}
			// Check 1 - s if s != 0.5
			if s != 0.5 {
				iPos, err = cInput.frame.Transform(kinematics.InterpolateValues(cInput.startInput, cInput.endInput, s))
				if !oFunc(iPos) || err != nil {
					return false
				}
			}
		}
		return true
	}
}

