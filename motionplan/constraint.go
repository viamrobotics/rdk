package motionplan

import (
	"errors"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/core/kinematics"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// ConstraintInput contains all the information a constraint needs to determine validity for a movement.
// It contains the starting inputs, the ending inputs, corresponding poses, and the frame it refers to.
// Pose fields may be empty, and may be filled in by a constraint that needs them.
type ConstraintInput struct {
	startPos   spatial.Pose
	endPos     spatial.Pose
	startInput []frame.Input
	endInput   []frame.Input
	frame      frame.Frame
}

// Constraint defines a struct that contains a function which is able to determine whether or not a given position
// is valid.
// TODO (pl): Determine how Gradient should fit into this
type Constraint interface {
	// A bool returning whether the given input is known to be good, and a float representing how far the input is
	// from "ideal".
	Valid(*ConstraintInput) (bool, float64)
}

// CheckConstraintPath will interpolate between two joint inputs and check that `true` is returned for all constraints
// in all intermediate positions. If failing on an intermediate position, it will return that position.
func CheckConstraintPath(mp MotionPlanner, ci *ConstraintInput) (bool, *ConstraintInput) {
	// ensure we have cartesian positions
	err := resolveInput(ci)
	if err != nil {
		return false, nil
	}
	steps := getSteps(ci.startPos, ci.endPos, mp.Resolution())

	lastInterp := 0.

	for i := 1; i <= steps; i++ {
		interp := float64(i) / float64(steps)
		interpC, ok := interpolateInput(ci, lastInterp, interp)
		lastInterp = interp
		if !ok {
			return false, nil
		}

		pass, _ := mp.CheckConstraints(interpC)
		if !pass {
			if i > 1 {
				return false, interpC
			}
			// fail on start pos
			return false, nil
		}
	}
	// extra step to check the end
	interpC, ok := interpolateInput(ci, 1, 1)
	if !ok {
		return false, nil
	}

	pass, _ := mp.CheckConstraints(interpC)
	if !pass {
		return false, interpC
	}

	return true, nil
}

var checkSeq = []float64{0.5, 0.333, 0.25, 0.17}

// constraintHandler is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse
type constraintHandler struct {
	constraints map[string]Constraint
}

// AddConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *constraintHandler) AddConstraint(name string, cons Constraint) {
	if c.constraints == nil {
		c.constraints = map[string]Constraint{}
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
func (c *constraintHandler) CheckConstraints(cInput *ConstraintInput) (bool, float64) {
	score := 0.

	for _, cFunc := range c.constraints {
		pass, cScore := cFunc.Valid(cInput)
		if !pass {
			return false, math.Inf(1)
		}
		score += cScore
	}
	return true, score
}

func interpolationCheck(cInput *ConstraintInput, by, epsilon float64) bool {
	iPos, err := cInput.frame.Transform(frame.InterpolateInputs(cInput.startInput, cInput.endInput, by))
	if err != nil {
		return false
	}
	interp := spatial.Interpolate(cInput.startPos, cInput.endPos, by)
	dist := kinematics.SquaredNorm(spatial.PoseDelta(iPos, interp))
	return dist <= epsilon
}

type flexibleConstraint struct {
	validFunc func(cInput *ConstraintInput) (bool, float64)
}

func (c *flexibleConstraint) Valid(cInput *ConstraintInput) (bool, float64) {
	if c.validFunc != nil {
		return c.validFunc(cInput)
	}
	return true, 0
}

func (c *flexibleConstraint) setFunc(f func(cInput *ConstraintInput) (bool, float64)) {
	c.validFunc = f
}

// NewInterpolatingConstraint creates a constraint function from an arbitrary function that will decide if a given pose is valid.
// This function will check the given function at each point in checkSeq, and 1-point. If all constraints are satisfied,
// it will return true. If any intermediate pose violates the constraint, will return false.
func NewInterpolatingConstraint(epsilon float64) Constraint {
	c := &flexibleConstraint{}
	f := func(cInput *ConstraintInput) (bool, float64) {
		for _, s := range checkSeq {
			ok := interpolationCheck(cInput, s, epsilon)
			if !ok {
				return ok, 0
			}
			// Check 1 - s if s != 0.5
			if s != 0.5 {
				ok := interpolationCheck(cInput, 1-s, epsilon)
				if !ok {
					return ok, 0
				}
			}
		}
		return true, 0
	}
	c.setFunc(f)
	return c
}

// NewPoseConstraint enforces a constant pose
func NewPoseConstraint() func(*ConstraintInput) (bool, float64) {
	return func(cInput *ConstraintInput) (bool, float64) {
		if cInput.startPos != nil {
			oDiff := spatial.OrientationBetween(cInput.startPos.Orientation(), &spatial.OrientationVector{OZ: -1})
			r4 := oDiff.AxisAngles()
			dist := r3.Vector{r4.RX * r4.Theta, r4.RY * r4.Theta, r4.RZ * r4.Theta}.Norm()
			if dist > 0.01 {
				return false, dist
			}
		}
		if cInput.endPos != nil {
			oDiff := spatial.OrientationBetween(cInput.endPos.Orientation(), &spatial.OrientationVector{OZ: -1})
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
func NewJointScorer() Constraint {
	c := &flexibleConstraint{}
	f := func(cInput *ConstraintInput) (bool, float64) {
		jScore := 0.
		for i, f := range cInput.startInput {
			jScore += math.Abs(f.Value - cInput.endInput[i].Value)
		}
		return true, jScore
	}
	c.setFunc(f)
	return c
}

// DontHitPetersWallConstraint defines some obstacles that nothing should not intersect with
// TODO(pl): put this somewhere else, maybe in an example file or something
func DontHitPetersWallConstraint() Constraint {

	f := func(ci *ConstraintInput) (bool, float64) {
		checkPt := func(pose spatial.Pose) bool {
			pt := pose.Point()

			// wall in Peter's office
			if pt.Y < -536.8 {
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
		} else if ci.startInput != nil {
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
		} else if ci.endInput != nil {
			pos, err := ci.frame.Transform(ci.endInput)
			if err != nil {
				return false, 0
			}
			if !checkPt(pos) {
				return false, 0
			}
		}
		return true, 0
	}
	return &flexibleConstraint{f}
}

// FakeObstacle simulates an obstacle.
func FakeObstacle(ci *ConstraintInput) (bool, float64) {
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
	} else if ci.startInput != nil {
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
	} else if ci.endInput != nil {
		pos, err := ci.frame.Transform(ci.endInput)
		if err != nil {
			return false, 0
		}
		if !checkPt(pos) {
			return false, 0
		}
	}
	return true, 0
}

// orientDistToRegion will return a function which will tell you how far the unit sphere component of an orientation
// vector is from a region defined by a point and an arclength around it. The theta value of OV is disregarded.
// This is useful, for example, in defining the set of acceptable angles of attack for writing on a whiteboard.
func orientDistToRegion(goal spatial.Orientation, alpha float64) func(spatial.Orientation) float64 {
	ov1 := goal.OrientationVectorRadians()
	return func(o spatial.Orientation) float64 {
		ov2 := o.OrientationVectorRadians()
		dist := math.Acos(ov1.OX*ov2.OX + ov1.OY*ov2.OY + ov1.OZ*ov2.OZ)
		return math.Max(0, dist-alpha)
	}
}

// NewPoseFlexOVGradient will provide a gradient function which will converge on an OV within an arclength of `alpha`
// of the ov of the goal given. The 3d point of the goal given is discarded, and the function will converge on the
// 3d point of the `to` pose (this is probably what you want).
func NewPoseFlexOVGradient(goal spatial.Pose, alpha float64) func(spatial.Pose, spatial.Pose) float64 {
	oDistFunc := orientDistToRegion(goal.Orientation(), alpha)
	return func(from, to spatial.Pose) float64 {
		pDist := from.Point().Distance(to.Point())
		oDist := oDistFunc(from.Orientation())
		return pDist*pDist + oDist*oDist
	}
}

// NewPlaneConstraintAndGradient is used to define a constraint space for a plane, and will return 1) a constraint
// function which will determine whether a point is on the plane and in a valid orientation, and 2) a gradient function
// which will bring a pose into the valid constraint space. The plane normal is assumed to point towards the valid area.
// angle refers to the maximum unit sphere arc length deviation from the ov
// epsilon refers to the closeness to the plane necessary to be a valid pose
func NewPlaneConstraintAndGradient(pNorm, pt r3.Vector, writingAngle, epsilon float64) (Constraint, func(spatial.Pose, spatial.Pose) float64) {
	// get the constant value for the plane
	pConst := -pt.Dot(pNorm)

	// invert the normal to get the valid AOA OV
	ov := &spatial.OrientationVector{OX: -pNorm.X, OY: -pNorm.Y, OZ: -pNorm.Z}
	ov.Normalize()

	dFunc := orientDistToRegion(ov, writingAngle)

	// distance from plane to point
	planeDist := func(pt r3.Vector) float64 {
		return math.Abs(pNorm.Dot(pt) + pConst)
	}

	// TODO: do we need to care about trajectory here? Probably, but not yet implemented
	gradFunc := func(from, to spatial.Pose) float64 {
		pDist := planeDist(from.Point())
		oDist := dFunc(from.Orientation())
		return pDist*pDist + oDist*oDist
	}

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInput(cInput)
		if err != nil {
			return false, 0
		}
		dist := gradFunc(cInput.startPos, cInput.endPos)
		if dist < epsilon*epsilon {
			return true, 0
		}
		return false, 0
	}

	c := &flexibleConstraint{validFunc}

	return c, gradFunc
}

// NewLineConstraintAndGradient is used to define a constraint space for a line, and will return 1) a constraint
// function which will determine whether a point is on the line and in a valid orientation, and 2) a gradient function
// which will bring a pose into the valid constraint space. The OV passed in defines the center of the valid orientation area.
// angle refers to the maximum unit sphere arc length deviation from the ov
// epsilon refers to the closeness to the line necessary to be a valid pose
func NewLineConstraintAndGradient(pt1, pt2 r3.Vector, ov *spatial.OrientationVector, writingAngle, epsilon float64) (Constraint, func(spatial.Pose, spatial.Pose) float64) {
	// invert the normal to get the valid AOA OV
	ov.Normalize()

	dFunc := orientDistToRegion(ov, writingAngle)

	// distance from line to point
	lineDist := func(point r3.Vector) float64 {

		ab := pt1.Sub(pt2)
		av := point.Sub(pt2)

		if av.Dot(ab) <= 0.0 { // Point is lagging behind start of the segment, so perpendicular distance is not viable.
			return av.Norm() // Use distance to start of segment instead.
		}

		bv := point.Sub(pt1)

		if bv.Dot(ab) >= 0.0 { // Point is advanced past the end of the segment, so perpendicular distance is not viable.
			return bv.Norm()
		}
		dist := (ab.Cross(av)).Norm() / ab.Norm()

		return dist

	}

	gradFunc := func(from, to spatial.Pose) float64 {
		pDist := lineDist(from.Point())
		oDist := dFunc(from.Orientation())

		return pDist*pDist + oDist*oDist
	}

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInput(cInput)
		if err != nil {
			return false, 0
		}
		dist := gradFunc(cInput.startPos, cInput.endPos)
		if dist < epsilon*epsilon {
			return true, 0
		}
		return false, dist
	}

	c := &flexibleConstraint{validFunc}

	return c, gradFunc
}

// PositionOnlyGradient returns the point-wise distance between two poses without regard for orientation.
// This is useful for scenarios where there are not enough DOF to control orientation, but arbitrary spatial points may
// still be arived at.
func PositionOnlyGradient(from, to spatial.Pose) float64 {
	pDist := from.Point().Distance(to.Point())
	return pDist * pDist
}

// Given a constraint input with only frames and input positions, calculates the corresponding poses as needed.
func resolveInput(ci *ConstraintInput) error {
	if ci.startPos == nil {
		if ci.frame != nil {
			if ci.startInput != nil {
				pos, err := ci.frame.Transform(ci.startInput)
				if err == nil {
					ci.startPos = pos
				} else {
					return err
				}
			} else {
				return errors.New("invalid constraint input")
			}
		} else {
			return errors.New("invalid constraint input")
		}
	}
	if ci.endPos == nil {
		if ci.frame != nil {
			if ci.endInput != nil {
				pos, err := ci.frame.Transform(ci.endInput)
				if err == nil {
					ci.endPos = pos
				} else {
					return err
				}
			} else {
				return errors.New("invalid constraint input")
			}
		} else {
			return errors.New("invalid constraint input")
		}
	}
	return nil
}

// TODO: add spatial transforms
func interpolateInput(ci *ConstraintInput, by1, by2 float64) (*ConstraintInput, bool) {
	new := &ConstraintInput{}
	new.frame = ci.frame
	new.startInput = frame.InterpolateInputs(ci.startInput, ci.endInput, by1)
	new.endInput = frame.InterpolateInputs(ci.startInput, ci.endInput, by2)

	return new, true
}
