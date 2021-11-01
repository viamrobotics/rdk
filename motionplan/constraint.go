package motionplan

import (
	"math"
	"fmt"

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
	constraints map[string]func(constraintInput) (bool, float64)
}

// AddConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *constraintHandler) AddConstraint(name string, cons func(constraintInput) (bool, float64)) {
	if c.constraints == nil {
		c.constraints = map[string]func(constraintInput) (bool, float64){}
	}
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

func interpolationCheck(cInput constraintInput, s float64) bool {
	epsilon := 0.01
	
	//~ fmt.Println("j1", cInput.startInput)
	//~ fmt.Println("j2", cInput.endInput)
	
	iPos, err := cInput.frame.Transform(frame.InterpolateInputs(cInput.startInput, cInput.endInput, s))
	if err != nil {
		fmt.Println("err")
		return false
	}
	interp := spatial.Interpolate(cInput.startPos, cInput.endPos, s)
	//~ fmt.Println("1: ", spatial.PoseToArmPos(iPos))
	//~ fmt.Println("2: ", spatial.PoseToArmPos(interp))
	dist := kinematics.SquaredNorm(spatial.PoseDelta(iPos, interp))
	//~ fmt.Println(dist)
	if dist > epsilon {
		return false
	}
	return true
}

// NewInterpolatingConstraint creates a constraint function from an arbitrary function that will decide if a given pose is valid.
// This function will check the given function at each point in checkSeq, and 1-point. If all constraints are satisfied,
// it will return true. If any intermediate pose violates the constraint, will return false. 
func NewInterpolatingConstraint() func(constraintInput) (bool, float64) {
	return func(cInput constraintInput) (bool, float64){
		for _, s := range checkSeq {
			ok := interpolationCheck(cInput, s)
			if !ok {
				return ok, 0
			}
			// Check 1 - s if s != 0.5
			if s != 0.5 {
				ok := interpolationCheck(cInput, 1-s)
				if !ok {
					return ok, 0
				}
			}
		}
		return true, 0
	}
}

// NewJointScorer returns a function which will sum the differences in each input from start to end
func NewJointScorer() func(constraintInput) (bool, float64) {
	return func(cInput constraintInput) (bool, float64){
		jScore := 0.
		for i, f := range cInput.startInput {
			jScore += math.Abs(f.Value - cInput.endInput[i].Value)
		}
		return true, jScore
	}
}
