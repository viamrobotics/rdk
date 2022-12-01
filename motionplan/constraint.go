package motionplan

import (
	"errors"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// ConstraintInput contains all the information a constraint needs to determine validity for a movement.
// It contains the starting inputs, the ending inputs, corresponding poses, and the frame it refers to.
// Pose fields may be empty, and may be filled in by a constraint that needs them.
type ConstraintInput struct {
	StartPos   spatial.Pose
	EndPos     spatial.Pose
	StartInput []referenceframe.Input
	EndInput   []referenceframe.Input
	Frame      referenceframe.Frame
}

// Constraint defines functions able to determine whether or not a given position is valid.
// TODO (pl): Determine how Gradient should fit into this
// A bool returning whether the given input is known to be good, and a float representing how far the input is
// from "ideal".
type Constraint func(*ConstraintInput) (bool, float64)

// constraintHandler is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type constraintHandler struct {
	constraints map[string]Constraint
}

// CheckConstraintPath will interpolate between two joint inputs and check that `true` is returned for all constraints
// in all intermediate positions. If failing on an intermediate position, it will return that position.
func (c *constraintHandler) CheckConstraintPath(ci *ConstraintInput, resolution float64) (bool, *ConstraintInput) {
	// ensure we have cartesian positions+
	err := resolveInputsToPositions(ci)
	if err != nil {
		return false, nil
	}
	steps := GetSteps(ci.StartPos, ci.EndPos, resolution)

	var lastGood []referenceframe.Input
	// Seed with just the start position to walk the path
	interpC := &ConstraintInput{Frame: ci.Frame}
	interpC.StartInput = ci.StartInput
	interpC.EndInput = ci.StartInput

	for i := 1; i <= steps; i++ {
		interp := float64(i) / float64(steps)
		interpC, err = cachedInterpolateInput(ci, interp, interpC.EndInput, interpC.EndPos)
		if err != nil {
			return false, nil
		}
		pass, _ := c.CheckConstraints(interpC)
		if !pass {
			if i > 1 {
				return false, &ConstraintInput{StartInput: lastGood, EndInput: interpC.StartInput}
			}
			// fail on start pos
			return false, nil
		}
		lastGood = interpC.StartInput
	}
	// extra step to check the end
	if err != nil {
		return false, nil
	}
	pass, _ := c.CheckConstraints(&ConstraintInput{
		StartPos:   ci.EndPos,
		EndPos:     ci.EndPos,
		StartInput: ci.EndInput,
		EndInput:   ci.EndInput,
		Frame:      ci.Frame,
	})
	if !pass {
		return false, &ConstraintInput{StartInput: lastGood, EndInput: interpC.StartInput}
	}

	return true, nil
}

// AddConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *constraintHandler) AddConstraint(name string, cons Constraint) {
	if c.constraints == nil {
		c.constraints = map[string]Constraint{}
	}
	if cons != nil {
		c.constraints[name] = cons
	}
}

// RemoveConstraint will remove the given constraint.
func (c *constraintHandler) RemoveConstraint(name string) {
	delete(c.constraints, name)
}

// Constraints will list all constraints by name.
func (c *constraintHandler) Constraints() []string {
	names := make([]string, 0, len(c.constraints))
	for name := range c.constraints {
		names = append(names, name)
	}
	return names
}

// CheckConstraints will check a given input against all constraints.
func (c *constraintHandler) CheckConstraints(cInput *ConstraintInput) (bool, float64) {
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

// NewCollisionConstraint is a helper function for creating a collision Constraint that takes a frame and geometries
// representing obstacles and interaction spaces and will construct a collision avoidance constraint from them.
func NewCollisionConstraint(
	frame referenceframe.Frame,
	goodInput []referenceframe.Input,
	obstacles, interactionSpaces map[string]spatial.Geometry,
	reportDistances bool,
) Constraint {
	zeroVols, err := frame.Geometries(goodInput)
	if err != nil && len(zeroVols.Geometries()) == 0 {
		return nil // no geometries defined for frame
	}
	internalEntities, err := NewObjectCollisionEntities(zeroVols.Geometries())
	if err != nil {
		return nil
	}
	obstacleEntities, err := NewObjectCollisionEntities(obstacles)
	if err != nil {
		return nil
	}
	spaceEntities, err := NewSpaceCollisionEntities(interactionSpaces)
	if err != nil {
		return nil
	}
	zeroCG, err := NewCollisionSystem(internalEntities, []CollisionEntities{obstacleEntities, spaceEntities}, true)
	if err != nil {
		return nil
	}

	constraint := func(cInput *ConstraintInput) (bool, float64) {
		internal, err := cInput.Frame.Geometries(cInput.StartInput)
		if err != nil && internal == nil {
			return false, 0
		}
		internalEntities, err := NewObjectCollisionEntities(internal.Geometries())
		if err != nil {
			return false, 0
		}

		cg, err := NewCollisionSystemFromReference(
			internalEntities,
			[]CollisionEntities{obstacleEntities, spaceEntities},
			zeroCG,
			reportDistances,
		)
		if err != nil {
			return false, 0
		}

		collisions := cg.Collisions()
		if len(collisions) > 0 {
			return false, 0
		}
		if !reportDistances {
			return true, 0
		}
		sum := 0.
		for _, collision := range collisions {
			sum += collision.penetrationDepth
		}
		return true, sum
	}
	return constraint
}

// NewCollisionConstraintFromWorldState creates a collision constraint from a world state, framesystem, a model and a set of initial states.
func NewCollisionConstraintFromWorldState(
	frame referenceframe.Frame,
	fs referenceframe.FrameSystem,
	worldState *referenceframe.WorldState,
	observationInput map[string][]referenceframe.Input,
	reportDistances bool,
) (Constraint, error) {
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldState, err := worldState.ToWorldFrame(fs, observationInput)
	if err != nil {
		return nil, err
	}

	// extract inputs corresponding to the frame
	var goodInputs []referenceframe.Input
	switch f := frame.(type) {
	case *solverFrame:
		goodInputs, err = f.mapToSlice(observationInput)
	default:
		goodInputs, err = referenceframe.GetFrameInputs(f, observationInput)
	}
	if err != nil {
		return nil, err
	}

	return NewCollisionConstraint(
		frame,
		goodInputs,
		worldState.Obstacles[0].Geometries(),
		worldState.InteractionSpaces[0].Geometries(),
		reportDistances,
	), nil
}

// NewAbsoluteLinearInterpolatingConstraint provides a Constraint whose valid manifold allows a specified amount of deviation from the
// shortest straight-line path between the start and the goal. linTol is the allowed linear deviation in mm, orientTol is the allowed
// orientation deviation measured by norm of the R3AA orientation difference to the slerp path between start/goal orientations.
func NewAbsoluteLinearInterpolatingConstraint(from, to spatial.Pose, linTol, orientTol float64) (Constraint, Metric) {
	orientConstraint, orientMetric := NewSlerpOrientationConstraint(from, to, orientTol)
	lineConstraint, lineMetric := NewLineConstraint(from.Point(), to.Point(), linTol)
	interpMetric := CombineMetrics(orientMetric, lineMetric)

	f := func(cInput *ConstraintInput) (bool, float64) {
		oValid, oDist := orientConstraint(cInput)
		lValid, lDist := lineConstraint(cInput)
		return oValid && lValid, oDist + lDist
	}
	return f, interpMetric
}

// NewProportionalLinearInterpolatingConstraint will provide the same metric and constraint as NewAbsoluteLinearInterpolatingConstraint,
// except that allowable linear and orientation deviation is scaled based on the distance from start to goal.
func NewProportionalLinearInterpolatingConstraint(from, to spatial.Pose, epsilon float64) (Constraint, Metric) {
	orientTol := epsilon * orientDist(from.Orientation(), to.Orientation())
	linTol := epsilon * from.Point().Distance(to.Point())

	return NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
}

// NewJointConstraint returns a constraint which will sum the squared differences in each input from start to end
// It will return false if that sum is over a specified threshold.
func NewJointConstraint(threshold float64) Constraint {
	f := func(cInput *ConstraintInput) (bool, float64) {
		jScore := 0.
		for i, f := range cInput.StartInput {
			jScore += math.Abs(f.Value - cInput.EndInput[i].Value)
		}
		return jScore < threshold, jScore
	}
	return f
}

// NewOrientationConstraint returns a constraint which will return false if the startPos or endPos orientations
// are not valid.
func NewOrientationConstraint(orientFunc func(o spatial.Orientation) bool) Constraint {
	f := func(cInput *ConstraintInput) (bool, float64) {
		if cInput.StartPos == nil || cInput.EndPos == nil {
			err := resolveInputsToPositions(cInput)
			if err != nil {
				return false, 0
			}
		}
		if orientFunc(cInput.StartPos.Orientation()) && orientFunc(cInput.EndPos.Orientation()) {
			return true, 0
		}
		return false, 0
	}
	return f
}

// NewSlerpOrientationConstraint will measure the orientation difference between the orientation of two poses, and return a constraint that
// returns whether a given orientation is within a given tolerance distance of the shortest arc between the two orientations, as well as a
// metric which returns the distance to that valid region.
func NewSlerpOrientationConstraint(start, goal spatial.Pose, tolerance float64) (Constraint, Metric) {
	var gradFunc func(from, _ spatial.Pose) float64
	origDist := math.Max(orientDist(start.Orientation(), goal.Orientation()), defaultEpsilon)

	gradFunc = func(from, _ spatial.Pose) float64 {
		sDist := orientDist(start.Orientation(), from.Orientation())
		gDist := 0.

		// If origDist is less than or equal to defaultEpsilon, then the starting and ending orientations are the same and we do not need
		// to compute the distance to the ending orientation
		if origDist > defaultEpsilon {
			gDist = orientDist(goal.Orientation(), from.Orientation())
		}
		return (sDist + gDist) - origDist
	}

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInputsToPositions(cInput)
		if err != nil {
			return false, 0
		}
		dist := gradFunc(cInput.StartPos, cInput.EndPos)
		if dist < tolerance {
			return true, 0
		}
		return false, 0
	}

	return validFunc, gradFunc
}

// NewPlaneConstraint is used to define a constraint space for a plane, and will return 1) a constraint
// function which will determine whether a point is on the plane and in a valid orientation, and 2) a distance function
// which will bring a pose into the valid constraint space. The plane normal is assumed to point towards the valid area.
// angle refers to the maximum unit sphere arc length deviation from the ov
// epsilon refers to the closeness to the plane necessary to be a valid pose.
func NewPlaneConstraint(pNorm, pt r3.Vector, writingAngle, epsilon float64) (Constraint, Metric) {
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
	gradFunc := func(from, _ spatial.Pose) float64 {
		pDist := planeDist(from.Point())
		oDist := dFunc(from.Orientation())
		return pDist*pDist + oDist*oDist
	}

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInputsToPositions(cInput)
		if err != nil {
			return false, 0
		}
		dist := gradFunc(cInput.StartPos, cInput.EndPos)
		if dist < epsilon*epsilon {
			return true, 0
		}
		return false, 0
	}

	return validFunc, gradFunc
}

// NewLineConstraint is used to define a constraint space for a line, and will return 1) a constraint
// function which will determine whether a point is on the line, and 2) a distance function
// which will bring a pose into the valid constraint space.
// tolerance refers to the closeness to the line necessary to be a valid pose in mm.
func NewLineConstraint(pt1, pt2 r3.Vector, tolerance float64) (Constraint, Metric) {
	// distance from line to point
	distToLine := func(point r3.Vector) float64 {
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

	if pt1.Distance(pt2) < defaultEpsilon {
		tolerance = defaultEpsilon
	}

	gradFunc := func(from, _ spatial.Pose) float64 {
		pDist := math.Max(distToLine(from.Point())-tolerance, 0)
		return pDist
	}

	validFunc := func(cInput *ConstraintInput) (bool, float64) {
		err := resolveInputsToPositions(cInput)
		if err != nil {
			return false, 0
		}
		dist := gradFunc(cInput.StartPos, cInput.EndPos)
		if dist == 0 {
			return true, 0
		}
		return false, dist
	}

	return validFunc, gradFunc
}

// NewPositionOnlyMetric returns a Metric that reports the point-wise distance between two poses.
func NewPositionOnlyMetric() Metric {
	return positionOnlyDist
}

// positionOnlyDist returns the point-wise distance between two poses without regard for orientation.
// This is useful for scenarios where there are not enough DOF to control orientation, but arbitrary spatial points may
// still be arived at.
func positionOnlyDist(from, to spatial.Pose) float64 {
	pDist := from.Point().Distance(to.Point())
	return pDist * pDist
}

// Given a constraint input with only frames and input positions, calculates the corresponding poses as needed.
func resolveInputsToPositions(ci *ConstraintInput) error {
	if ci.StartPos == nil {
		if ci.Frame != nil {
			if ci.StartInput != nil {
				pos, err := ci.Frame.Transform(ci.StartInput)
				if err == nil {
					ci.StartPos = pos
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
	if ci.EndPos == nil {
		if ci.Frame != nil {
			if ci.EndInput != nil {
				pos, err := ci.Frame.Transform(ci.EndInput)
				if err == nil {
					ci.EndPos = pos
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

// Prevents recalculation of startPos. If no startPos has been calculated, just pass nil.
func cachedInterpolateInput(
	ci *ConstraintInput,
	by float64,
	startInput []referenceframe.Input,
	startPos spatial.Pose,
) (*ConstraintInput, error) {
	input := &ConstraintInput{}
	input.Frame = ci.Frame
	input.StartInput = startInput
	input.StartPos = startPos
	input.EndInput = referenceframe.InterpolateInputs(ci.StartInput, ci.EndInput, by)

	return input, resolveInputsToPositions(input)
}
