package motionplan

import (
	"errors"
	"math"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/service/motion/v1"

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
	steps := PathStepCount(ci.StartPos, ci.EndPos, resolution)

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
		pass, _, _ := c.CheckConstraints(interpC)
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
	pass, _, _ := c.CheckConstraints(&ConstraintInput{
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

// AddConstraints will add or overwrite constraint functions with the ones present in the specified map.
// A constraint function should return true if the given position satisfies the constraint.
func (c *constraintHandler) AddConstraints(constraints map[string]Constraint) {
	for name, constraint := range constraints {
		c.AddConstraint(name, constraint)
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
// Return values are:
// -- a bool representing whether all constraints passed
// -- if passing, a score representing the distance to a non-passing state. Inf(1) if failing.
// -- if failing, a string naming the failed constraint.
func (c *constraintHandler) CheckConstraints(cInput *ConstraintInput) (bool, float64, string) {
	score := 0.

	for name, cFunc := range c.constraints {
		pass, cScore := cFunc(cInput)
		if !pass {
			return false, math.Inf(1), name
		}
		score += cScore
	}
	return true, score, ""
}

func newCollisionConstraints(
	frame *solverFrame,
	fs referenceframe.FrameSystem,
	worldState *referenceframe.WorldState,
	inputs map[string][]referenceframe.Input,
	pbConstraint []*pb.CollisionSpecification,
	reportDistances bool,
) (map[string]Constraint, error) {
	// extract inputs corresponding to the frame
	frameInputs, err := frame.mapToSlice(inputs)
	if err != nil {
		return nil, err
	}

	// create robot collision entities
	movingGeometries, err := frame.Geometries(frameInputs)
	if err != nil && len(movingGeometries.Geometries()) == 0 {
		return nil, err // no geometries defined for frame
	}

	// find all geoemetries that are not moving but are in the frame system
	staticGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, inputs)
	if err != nil {
		return nil, err
	}
	for name, geometries := range frameSystemGeometries {
		if !frame.movingFrame(name) {
			staticGeometries = append(staticGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	obstacles, err := worldState.ObstaclesInWorldFrame(fs, inputs)
	if err != nil {
		return nil, err
	}

	allowedCollisions, err := collisionSpecificationsFromProto(pbConstraint, frameSystemGeometries, worldState)
	if err != nil {
		return nil, err
	}

	// create constraint to keep moving geometries from hitting world state obstacles
	// can use zeroth element of worldState.Obstacles because ToWorldFrame returns only one GeometriesInFrame
	obstacleConstraint, err := newCollisionConstraint(
		movingGeometries.Geometries(),
		obstacles.Geometries(),
		allowedCollisions,
		reportDistances,
	)
	if err != nil {
		return nil, err
	}

	// create constraint to keep moving geometries from hitting other geometries on robot that are not moving
	robotConstraint, err := newCollisionConstraint(movingGeometries.Geometries(), staticGeometries, allowedCollisions, reportDistances)
	if err != nil {
		return nil, err
	}

	// create constraint to keep moving geometires from hitting themselves
	selfCollisionConstraint, err := newCollisionConstraint(movingGeometries.Geometries(), nil, allowedCollisions, reportDistances)
	if err != nil {
		return nil, err
	}

	return map[string]Constraint{
		defaultObstacleConstraintName:       obstacleConstraint,
		defaultSelfCollisionConstraintName:  selfCollisionConstraint,
		defaultRobotCollisionConstraintName: robotConstraint,
	}, nil
}

// newCollisionConstraint is the most general method to create a collision constraint, which will be violated if geometries constituting
// the given frame ever come into collision with obstacle geometries outside of the collisions present for the observationInput.
// Collisions specified as collisionSpecifications will also be ignored
// if reportDistances is false, this check will be done as fast as possible, if true maximum information will be available for debugging.
func newCollisionConstraint(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
) (Constraint, error) {
	// create the reference collisionGraph
	zeroCG, err := newCollisionGraph(moving, static, nil, true)
	if err != nil {
		return nil, err
	}
	for _, specification := range collisionSpecifications {
		zeroCG.addCollisionSpecification(specification)
	}

	// create constraint from reference collision graph
	constraint := func(cInput *ConstraintInput) (bool, float64) {
		internal, err := cInput.Frame.Geometries(cInput.StartInput)
		if err != nil && internal == nil {
			return false, 0
		}

		cg, err := newCollisionGraph(internal.Geometries(), static, zeroCG, reportDistances)
		if err != nil {
			return false, 0
		}

		collisions := cg.collisions()
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
	return constraint, nil
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
	if pt1.Distance(pt2) < defaultEpsilon {
		tolerance = defaultEpsilon
	}

	gradFunc := func(from, _ spatial.Pose) float64 {
		pDist := math.Max(spatial.DistToLineSegment(pt1, pt2, from.Point())-tolerance, 0)
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
