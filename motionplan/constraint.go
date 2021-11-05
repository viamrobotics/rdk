package motionplan

import (
	"math"
	"fmt"

	"github.com/golang/geo/r3"
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

// TODO: add spatial transforms
func interpolateInput(ci constraintInput, by float64) (constraintInput, bool){
	var new constraintInput
	new.frame = ci.frame
	new.startInput = ci.startInput
	new.endInput = frame.InterpolateInputs(ci.startInput, ci.endInput, by)
	
	return new, true
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

// CheckConstraints will check a given input against all constraints
func (c *constraintHandler) CheckConstraints(cInput constraintInput) (bool, float64) {
	score := 0.
	for _, cFunc := range c.constraints {
		pass, cScore := cFunc(cInput)
		if !pass {
			return false, math.Inf(1)
		}
		score += cScore
	}
	return true, score
}

// CheckConstraintPath will interpolate between two joint inputs and check that `true` is returned for all constraints
// in all intermediate positions
func (c *constraintHandler) CheckConstraintPath(ci constraintInput) bool {
	
	var seedPos, goalPos spatial.Pose
	var err error
	if ci.startPos != nil {
		seedPos = ci.startPos
	}else if ci.startInput != nil {
		seedPos, err = ci.frame.Transform(ci.startInput)
		if err != nil {
			return false
		}
		ci.startPos = seedPos
	}else{
		return false
	}
	if ci.endPos != nil {
		goalPos = ci.endPos
	}else if ci.startInput != nil {
		goalPos, err = ci.frame.Transform(ci.endInput)
		
		if err != nil {
			return false
		}
		ci.endPos = goalPos
	}else{
		return false
	}
	steps := getSteps(seedPos, goalPos)
	
	for i := 0; i < steps; i++ {
		interp := float64(i)/float64(steps)
		interpC, ok := interpolateInput(ci, interp)
		if !ok {
			return false
		}
		for _, cFunc := range c.constraints {
			pass, _ := cFunc(interpC)
			if !pass {
				return false
			}
		}
	}
	return true
}

func interpolationCheck(cInput constraintInput, s float64) bool {
	epsilon := 0.01
	
	iPos, err := cInput.frame.Transform(frame.InterpolateInputs(cInput.startInput, cInput.endInput, s))
	if err != nil {
		fmt.Println("err")
		return false
	}
	interp := spatial.Interpolate(cInput.startPos, cInput.endPos, s)
	dist := kinematics.SquaredNorm(spatial.PoseDelta(iPos, interp))
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

// NewPoseConstraint enforces a constant pose
func NewPoseConstraint() func(constraintInput) (bool, float64) {
	return func(cInput constraintInput) (bool, float64){
		if cInput.startPos != nil {
			oDiff := spatial.OrientationBetween(cInput.startPos.Orientation(), &spatial.OrientationVector{OZ:-1})
			r4 := oDiff.AxisAngles()
			dist := r3.Vector{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}.Norm()
			if dist > 0.01 {
				return false, dist
			}
		}
		if cInput.endPos != nil {
			oDiff := spatial.OrientationBetween(cInput.endPos.Orientation(), &spatial.OrientationVector{OZ:-1})
			r4 := oDiff.AxisAngles()
			dist := r3.Vector{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}.Norm()
			if dist > 0.01 {
				return false, dist
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

// Simulates an obstacle. Also makes the xArm7 in Peter's office not hit things
func fakeObstacle(ci constraintInput) (bool, float64) {
	checkPt := func(pose spatial.Pose) bool {
		pt := pose.Point()
		
		// cardboard box
		if pt.X > 275 && pt.X < 420 {
			if pt.Y < 310 && pt.Y > -310 {
				if pt.Z < 275 {
					return false
				}
			}
		}
		// wall in Peter's office
		if pt.Y < -390 {
			return false
		}
		if pt.X < -600 {
			return false
		}
		// shelf in Peter's office
		if pt.Z < 5 && pt.Y < 260 && pt.X < 140 {
			return false
		}
		
		return true
	}
	if ci.startPos != nil {
		if !checkPt(ci.startPos) {
			return false, 0
		}
	}else if ci.startInput != nil {
		pos, err := ci.frame.Transform(ci.startInput)
		if err != nil {
			return false, 0
		}
		if !checkPt(pos) {
			return false, 0
		}
	}
	if ci.endPos != nil {
		if !checkPt(ci.endPos) {
			return false, 0
		}
	}else if ci.endInput != nil {
		pos, err := ci.frame.Transform(ci.endInput)
		if err != nil {
			return false, 0
		}
		if !checkPt(pos) {
			return false, 0
		}
	}
	return true , 0
}

// force OV of OZ=-1
func constantOrient(d float64) func(spatial.Pose, spatial.Pose) float64 {
	
	return func(from, to spatial.Pose) float64{
		// allow 5mm deviation from `to` to allow orientation better
		dist := from.Point().Distance(to.Point())
		if dist < d {
			dist = 0
		}
		
		oDiff := spatial.OrientationBetween(from.Orientation(), &spatial.OrientationVector{OZ:-1})
		r4 := oDiff.AxisAngles()
		dist += r3.Vector{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}.Norm()
		
		return dist
	}
}
