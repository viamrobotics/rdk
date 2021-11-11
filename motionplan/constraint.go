package motionplan

import (
	"math"
	"fmt"
	"errors"

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
	new.startInput = frame.InterpolateInputs(ci.startInput, ci.endInput, by)
	new.endInput = ci.endInput
	
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
	
	//~ fmt.Println("checking", cInput)
	
	for name, cFunc := range c.constraints {
		pass, cScore := cFunc(cInput)
		//~ fmt.Println(name, pass)
		if !pass {
			fmt.Println(name, "failed, off by", cScore)
			return false, math.Inf(1)
		}
		score += cScore
	}
	return true, score
}

// CheckConstraintPath will interpolate between two joint inputs and check that `true` is returned for all constraints
// in all intermediate positions
func (c *constraintHandler) CheckConstraintPath(ci constraintInput) bool {
	//~ fmt.Println("path checking", ci)
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
	
	fmt.Println("steps:", steps, "from", spatial.PoseToArmPos(seedPos), "to", spatial.PoseToArmPos(goalPos))
	
	for i := 0; i < steps; i++ {
		interp := float64(i)/float64(steps)
		interpC, ok := interpolateInput(ci, interp)
		if !ok {
			return false
		}
		
		//~ fmt.Println("interp", interpC)
		pass, _ := c.CheckConstraints(interpC)
		if !pass {
			return false
		}
		
		//~ for _, cFunc := range c.constraints {
			//~ pass, _ := cFunc(interpC)
			//~ if !pass {
				//~ return false
			//~ }
		//~ }
	}
	return true
}

func interpolationCheck(cInput constraintInput, s float64) bool {
	epsilon := 0.1
	
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

// Simulates an obstacle.
func dontHitPetersWall(ci constraintInput) (bool, float64) {
	checkPt := func(pose spatial.Pose) bool {
		pt := pose.Point()
		
		// wall in Peter's office
		// this has some buffer- whiteboard at precisely -506
		if pt.Y < -495.8 {
			fmt.Println(spatial.PoseToArmPos(pose))
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

// Simulates an obstacle.
func fakeObstacle(ci constraintInput) (bool, float64) {
	checkPt := func(pose spatial.Pose) bool {
		pt := pose.Point()

		// wood panel box
		if pt.X > -290 && pt.X < 510 {
			if pt.Y < 500 && pt.Y > 200 {
				if pt.Z < 260 {
					return false
				}
			}
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
		// allow `d` mm deviation from `to` to allow orientation better
		dist := from.Point().Distance(to.Point())
		if dist < d {
			dist = 0
		}
		
		toGoal := spatial.NewPoseFromOrientation(to.Point(), &spatial.OrientationVector{OZ:-1})
		
		return kinematics.SquaredNorm(spatial.PoseDelta(from, toGoal))
		
		//~ oDiff := spatial.OrientationBetween(from.Orientation(), &spatial.OrientationVector{OZ:-1})
		//~ r4 := oDiff.AxisAngles()
		//~ dist += r3.Vector{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}.Norm()
		//~ fmt.Println(dist)
		//~ return dist
	}
}

// orientDistToRegion will return a function which will tell you how far the unit sphere component of an orientation
// vector is from a region defined by a point and an arclength around it. The theta value of OV is disregarded.
// This is useful, for example, in defining the set of acceptable angles of attack for writing on a whiteboard.
func orientDistToRegion(goal spatial.Orientation, alpha float64) func(spatial.Orientation) float64 {
	ov1 := goal.OrientationVectorRadians()
	return func(o spatial.Orientation) float64 {
		ov2 := o.OrientationVectorRadians()
		dist := math.Acos(ov1.OX*ov2.OX + ov1.OY*ov2.OY + ov1.OZ*ov2.OZ)
		return math.Max(0, dist - alpha)
	}
}


func NewPoseFlexOVGradient(goal spatial.Pose, alpha float64) func(spatial.Pose, spatial.Pose) float64 {
	
	oDistFunc := orientDistToRegion(goal.Orientation(), alpha)
	
	return func(from, to spatial.Pose) float64 {
		pDist := from.Point().Distance(to.Point())
		oDist := oDistFunc(from.Orientation())
		// pDist is already squared
		return pDist*pDist + oDist*oDist
	}
}

// NewPlaneConstraintAndGradient is used to define a constraint space for a plane, and will return 1) a constraint
// function which will determine whether a point is on the plane and in a valid orientation, and 2) a gradient function
// which will bring a pose into the valid constraint space. The plane normal is assumed to point towards the valid area
func NewPlaneConstraintAndGradient(pNorm, pt r3.Vector) (func(constraintInput) (bool, float64), func(spatial.Pose, spatial.Pose) float64) {
	
	// arc length from plane-perpendicular vector allowable for writing
	writingAngle := 0.3
	epsilon := 0.01
	
	// get the constant value for the plane
	pConst := -pt.Dot(pNorm)
	
	// invert the normal to get the valid AOA OV
	ov := &spatial.OrientationVector{OX:-pNorm.X, OY:-pNorm.Y, OZ:-pNorm.Z}
	ov.Normalize()
	
	dFunc := orientDistToRegion(ov, writingAngle)
	
	// distance from plane to point
	planeDist := func(pt r3.Vector) float64 {
		return math.Abs(pNorm.Dot(pt) + pConst)
	}
	
	// TODO: do we need to care about trajectory here? Probably, but not yet implemented
	gradFunc := func(from, to spatial.Pose) float64 {
		pDist := planeDist(from.Point())
		
		//~ fmt.Println(pDist, from.Point())
		
		oDist := dFunc(from.Orientation())
		return pDist*pDist + oDist*oDist
	}
	
	validFunc := func(cInput constraintInput) (bool, float64) {
		cInput, err := resolveInput(cInput)
		if err != nil{
			return false, 0
		}
		dist := gradFunc(cInput.startPos, cInput.endPos)
		if dist < epsilon*epsilon {
			return true, 0
		}
		return false, 0
	}
	
	return validFunc, gradFunc
}

// NewLineConstraintAndGradient is used to define a constraint space for a line, and will return 1) a constraint
// function which will determine whether a point is on the line and in a valid orientation, and 2) a gradient function
// which will bring a pose into the valid constraint space. The OV passed in defines the center of the valid orientation area.
func NewLineConstraintAndGradient(pt1, pt2 r3.Vector, ov *spatial.OrientationVector) (func(constraintInput) (bool, float64), func(spatial.Pose, spatial.Pose) float64) {
	
	// arc length from plane-perpendicular vector allowable for writing
	writingAngle := 0.4
	epsilon := 0.3
	
	// invert the normal to get the valid AOA OV
	ov.Normalize()
	
	dFunc := orientDistToRegion(ov, writingAngle)
	
	// distance from line to point
	lineDist := func(point r3.Vector) float64 {
		
		ab := pt1.Sub(pt2)
		av := point.Sub(pt2)

		if (av.Dot(ab) <= 0.0){           // Point is lagging behind start of the segment, so perpendicular distance is not viable.
			return av.Norm()     // Use distance to start of segment instead.
		}

		bv := point.Sub(pt1)

		if (bv.Dot(ab) >= 0.0){     // Point is advanced past the end of the segment, so perpendicular distance is not viable.
			return bv.Norm() 
		}
		dist := (ab.Cross(av)).Norm() / ab.Norm() 
		
		return dist

	}
	
	// TODO: do we need to care about trajectory here? Probably, but not yet implemented
	gradFunc := func(from, to spatial.Pose) float64 {
		pDist := lineDist(from.Point())
		oDist := dFunc(from.Orientation())
		
		fmt.Println("p, o", pDist, oDist)
		
		return pDist*pDist + oDist*oDist
	}
	
	validFunc := func(cInput constraintInput) (bool, float64) {
		cInput, err := resolveInput(cInput)
		if err != nil{
			return false, 0
		}
		dist := gradFunc(cInput.startPos, cInput.endPos)
		if dist < epsilon*epsilon {
			//~ fmt.Println(dist)
			return true, 0
		}
		return false, dist
	}
	
	return validFunc, gradFunc
}

func resolveInput(ci constraintInput) (constraintInput, error) {
	if ci.startPos == nil {
		if ci.frame != nil {
			if ci.startInput != nil {
				pos, err := ci.frame.Transform(ci.startInput)
				if err == nil {
					ci.startPos = pos
				}else{
					return ci, err
				}
			}else{
				return ci, errors.New("invalid constraint input")
			}
		}else{
			return ci, errors.New("invalid constraint input")
		}
	}
	if ci.endPos == nil {
		if ci.frame != nil {
			if ci.endInput != nil {
				pos, err := ci.frame.Transform(ci.endInput)
				if err == nil {
					ci.endPos = pos
				}else{
					return ci, err
				}
			}else{
				return ci, errors.New("invalid constraint input")
			}
		}else{
			return ci, errors.New("invalid constraint input")
		}
	}
	return ci, nil
}
