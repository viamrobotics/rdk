//go:build !no_cgo

package motionplan

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	motionpb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

var defaultMinStepCount = 2

// Given a constraint input with only frames and input positions, calculates the corresponding poses as needed.
func resolveSegmentsToPositions(segment *ik.Segment) error {
	if segment.StartPosition == nil {
		if segment.Frame != nil {
			if segment.StartConfiguration != nil {
				pos, err := segment.Frame.Transform(segment.StartConfiguration)
				if err == nil {
					segment.StartPosition = pos
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
	if segment.EndPosition == nil {
		if segment.Frame != nil {
			if segment.EndConfiguration != nil {
				pos, err := segment.Frame.Transform(segment.EndConfiguration)
				if err == nil {
					segment.EndPosition = pos
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

// Given a constraint input with only frames and input positions, calculates the corresponding poses as needed.
func resolveStatesToPositions(state *ik.State) error {
	if state.Position == nil {
		if state.Frame != nil {
			if state.Configuration != nil {
				pos, err := state.Frame.Transform(state.Configuration)
				if err == nil {
					state.Position = pos
				} else {
					return err
				}
			} else {
				return errInvalidConstraint
			}
		} else {
			return errInvalidConstraint
		}
	}
	return nil
}

// SegmentFSConstraint tests whether a transition from a starting robot configuration to an ending robot configuration is valid.
// If the returned bool is true, the constraint is satisfied and the segment is valid.
type SegmentFSConstraint func(*ik.SegmentFS) bool

// SegmentConstraint tests whether a transition from a starting robot configuration to an ending robot configuration is valid.
// If the returned bool is true, the constraint is satisfied and the segment is valid.
type SegmentConstraint func(*ik.Segment) bool

// StateFSConstraint tests whether a given robot configuration is valid
// If the returned bool is true, the constraint is satisfied and the state is valid.
type StateFSConstraint func(*ik.StateFS) bool

// StateConstraint tests whether a given robot configuration is valid
// If the returned bool is true, the constraint is satisfied and the state is valid.
type StateConstraint func(*ik.State) bool

// ConstraintHandler is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type ConstraintHandler struct {
	segmentConstraints   map[string]SegmentConstraint
	segmentFSConstraints map[string]SegmentFSConstraint
	stateConstraints     map[string]StateConstraint
	stateFSConstraints   map[string]StateFSConstraint
}

// CheckStateConstraints will check a given input against all state constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckStateConstraints(state *ik.State) (bool, string) {
	for name, cFunc := range c.stateConstraints {
		pass := cFunc(state)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckStateFSConstraints will check a given input against all FS state constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckStateFSConstraints(state *ik.StateFS) (bool, string) {
	for name, cFunc := range c.stateFSConstraints {
		pass := cFunc(state)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckSegmentConstraints will check a given input against all segment constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckSegmentConstraints(segment *ik.Segment) (bool, string) {
	for name, cFunc := range c.segmentConstraints {
		pass := cFunc(segment)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckSegmentFSConstraints will check a given input against all FS segment constraints.
// Return values are:
// -- a bool representing whether all constraints passed
// -- if failing, a string naming the failed constraint.
func (c *ConstraintHandler) CheckSegmentFSConstraints(segment *ik.SegmentFS) (bool, string) {
	for name, cFunc := range c.segmentFSConstraints {
		pass := cFunc(segment)
		if !pass {
			return false, name
		}
	}
	return true, ""
}

// CheckStateConstraintsAcrossSegment will interpolate the given input from the StartInput to the EndInput, and ensure that all intermediate
// states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will return `true, nil`.
// If any constraints fail, this will return false, and an Segment representing the valid portion of the segment, if any. If no
// part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintHandler) CheckStateConstraintsAcrossSegment(ci *ik.Segment, resolution float64) (bool, *ik.Segment) {
	interpolatedConfigurations, err := interpolateSegment(ci, resolution)
	if err != nil {
		return false, nil
	}
	var lastGood []referenceframe.Input
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &ik.State{Frame: ci.Frame, Configuration: interpConfig}
		if resolveStatesToPositions(interpC) != nil {
			return false, nil
		}
		pass, _ := c.CheckStateConstraints(interpC)
		if !pass {
			if i == 0 {
				// fail on start pos
				return false, nil
			}
			return false, &ik.Segment{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood}
		}
		lastGood = interpC.Configuration
	}

	return true, nil
}

// interpolateSegment is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func interpolateSegment(ci *ik.Segment, resolution float64) ([][]referenceframe.Input, error) {
	// ensure we have cartesian positions
	if err := resolveSegmentsToPositions(ci); err != nil {
		return nil, err
	}

	steps := PathStepCount(ci.StartPosition, ci.EndPosition, resolution)
	if steps < defaultMinStepCount {
		// Minimum step count ensures we are not missing anything
		steps = defaultMinStepCount
	}

	var interpolatedConfigurations [][]referenceframe.Input
	for i := 0; i <= steps; i++ {
		interp := float64(i) / float64(steps)
		interpConfig, err := ci.Frame.Interpolate(ci.StartConfiguration, ci.EndConfiguration, interp)
		if err != nil {
			return nil, err
		}
		interpolatedConfigurations = append(interpolatedConfigurations, interpConfig)
	}
	return interpolatedConfigurations, nil
}

// interpolateSegmentFS is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func interpolateSegmentFS(ci *ik.SegmentFS, resolution float64) ([]map[string][]referenceframe.Input, error) {
	// Find the frame with the most steps by calculating steps for each frame
	maxSteps := defaultMinStepCount
	for frameName, startConfig := range ci.StartConfiguration {
		endConfig, exists := ci.EndConfiguration[frameName]
		if !exists {
			return nil, fmt.Errorf("frame %s exists in start config but not in end config", frameName)
		}

		// Get frame from FrameSystem
		frame := ci.FS.Frame(frameName)
		if frame != nil {
			return nil, fmt.Errorf("frame %s exists in start config but not in framesystem", frameName)
		}

		// Calculate positions for this frame's start and end configs
		startPos, err := frame.Transform(startConfig)
		if err != nil {
			return nil, err
		}
		endPos, err := frame.Transform(endConfig)
		if err != nil {
			return nil, err
		}

		// Calculate steps needed for this frame
		steps := PathStepCount(startPos, endPos, resolution)
		if steps > maxSteps {
			maxSteps = steps
		}
	}

	// Create interpolated configurations for all frames
	var interpolatedConfigurations []map[string][]referenceframe.Input
	for i := 0; i <= maxSteps; i++ {
		interp := float64(i) / float64(maxSteps)
		frameConfigs := make(map[string][]referenceframe.Input)

		// Interpolate each frame's configuration
		for frameName, startConfig := range ci.StartConfiguration {
			endConfig := ci.EndConfiguration[frameName]
			frame := ci.FS.Frame(frameName)

			interpConfig, err := frame.Interpolate(startConfig, endConfig, interp)
			if err != nil {
				return nil, err
			}
			frameConfigs[frameName] = interpConfig
		}

		interpolatedConfigurations = append(interpolatedConfigurations, frameConfigs)
	}

	return interpolatedConfigurations, nil
}

// CheckSegmentAndStateValidity will check an segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintHandler) CheckSegmentAndStateValidity(segment *ik.Segment, resolution float64) (bool, *ik.Segment) {
	valid, subSegment := c.CheckStateConstraintsAcrossSegment(segment, resolution)
	if !valid {
		if subSegment != nil {
			subSegmentValid, _ := c.CheckSegmentConstraints(subSegment)
			if subSegmentValid {
				return false, subSegment
			}
		}
		return false, nil
	}
	// all states are valid
	valid, _ = c.CheckSegmentConstraints(segment)
	return valid, nil
}

// AddStateConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddStateConstraint(name string, cons StateConstraint) {
	if c.stateConstraints == nil {
		c.stateConstraints = map[string]StateConstraint{}
	}
	c.stateConstraints[name] = cons
}

// RemoveStateConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveStateConstraint(name string) {
	delete(c.stateConstraints, name)
}

// StateConstraints will list all state constraints by name.
func (c *ConstraintHandler) StateConstraints() []string {
	names := make([]string, 0, len(c.stateConstraints))
	for name := range c.stateConstraints {
		names = append(names, name)
	}
	return names
}

// AddSegmentConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddSegmentConstraint(name string, cons SegmentConstraint) {
	if c.segmentConstraints == nil {
		c.segmentConstraints = map[string]SegmentConstraint{}
	}
	c.segmentConstraints[name] = cons
}

// RemoveSegmentConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveSegmentConstraint(name string) {
	delete(c.segmentConstraints, name)
}

// SegmentConstraints will list all segment constraints by name.
func (c *ConstraintHandler) SegmentConstraints() []string {
	names := make([]string, 0, len(c.segmentConstraints))
	for name := range c.segmentConstraints {
		names = append(names, name)
	}
	return names
}

// AddStateFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddStateFSConstraint(name string, cons StateFSConstraint) {
	if c.stateFSConstraints == nil {
		c.stateFSConstraints = map[string]StateFSConstraint{}
	}
	c.stateFSConstraints[name] = cons
}

// RemoveStateFSConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveStateFSConstraint(name string) {
	delete(c.stateFSConstraints, name)
}

// StateFSConstraints will list all FS state constraints by name.
func (c *ConstraintHandler) StateFSConstraints() []string {
	names := make([]string, 0, len(c.stateFSConstraints))
	for name := range c.stateFSConstraints {
		names = append(names, name)
	}
	return names
}

// AddSegmentFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintHandler) AddSegmentFSConstraint(name string, cons SegmentFSConstraint) {
	if c.segmentFSConstraints == nil {
		c.segmentFSConstraints = map[string]SegmentFSConstraint{}
	}
	c.segmentFSConstraints[name] = cons
}

// RemoveSegmentFSConstraint will remove the given constraint.
func (c *ConstraintHandler) RemoveSegmentFSConstraint(name string) {
	delete(c.segmentFSConstraints, name)
}

// SegmentFSConstraints will list all FS segment constraints by name.
func (c *ConstraintHandler) SegmentFSConstraints() []string {
	names := make([]string, 0, len(c.segmentFSConstraints))
	for name := range c.segmentFSConstraints {
		names = append(names, name)
	}
	return names
}

func createAllCollisionConstraints(
	movingRobotGeometries, staticRobotGeometries, worldGeometries, boundingRegions []spatial.Geometry,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) (map[string]StateConstraint, error) {
	constraintMap := map[string]StateConstraint{}
	var err error

	if len(worldGeometries) > 0 {
		// Check if a moving geometry is in collision with a pointcloud. If so, error.
		// TODO: This is not the most robust way to deal with this but is better than driving through walls.
		var zeroCG *collisionGraph
		for _, geom := range worldGeometries {
			if octree, ok := geom.(*pointcloud.BasicOctree); ok {
				if zeroCG == nil {
					zeroCG, err = setupZeroCG(movingRobotGeometries, worldGeometries, allowedCollisions, collisionBufferMM)
					if err != nil {
						return nil, err
					}
				}
				for _, collision := range zeroCG.collisions(collisionBufferMM) {
					if collision.name1 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name2)
					} else if collision.name2 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name1)
					}
				}
			}
		}

		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraint, err := NewCollisionConstraint(movingRobotGeometries, worldGeometries, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintMap[defaultObstacleConstraintDesc] = obstacleConstraint
	}

	if len(boundingRegions) > 0 {
		// create constraint to keep moving geometries within the defined bounding regions
		interactionSpaceConstraint := NewBoundingRegionConstraint(movingRobotGeometries, boundingRegions, collisionBufferMM)
		constraintMap[defaultBoundingRegionConstraintDesc] = interactionSpaceConstraint
	}

	if len(staticRobotGeometries) > 0 {
		// create constraint to keep moving geometries from hitting other geometries on robot that are not moving
		robotConstraint, err := NewCollisionConstraint(
			movingRobotGeometries,
			staticRobotGeometries,
			allowedCollisions,
			false,
			collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintMap[defaultRobotCollisionConstraintDesc] = robotConstraint
	}

	// create constraint to keep moving geometries from hitting themselves
	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraint, err := NewCollisionConstraint(movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintMap[defaultSelfCollisionConstraintDesc] = selfCollisionConstraint
	}
	return constraintMap, nil
}

func setupZeroCG(moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	collisionBufferMM float64,
) (*collisionGraph, error) {
	// create the reference collisionGraph
	zeroCG, err := newCollisionGraph(moving, static, nil, true, collisionBufferMM)
	if err != nil {
		return nil, err
	}
	for _, specification := range collisionSpecifications {
		zeroCG.addCollisionSpecification(specification)
	}
	return zeroCG, nil
}

// NewCollisionConstraint is the most general method to create a collision constraint, which will be violated if geometries constituting
// the given frame ever come into collision with obstacle geometries outside of the collisions present for the observationInput.
// Collisions specified as collisionSpecifications will also be ignored
// if reportDistances is false, this check will be done as fast as possible, if true maximum information will be available for debugging.
func NewCollisionConstraint(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (StateConstraint, error) {
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, collisionBufferMM)
	if err != nil {
		return nil, err
	}

	// create constraint from reference collision graph
	constraint := func(state *ik.State) bool {
		var internalGeoms []spatial.Geometry
		switch {
		case state.Configuration != nil:
			internal, err := state.Frame.Geometries(state.Configuration)
			if err != nil {
				return false
			}
			internalGeoms = internal.Geometries()
		case state.Position != nil:
			// TODO(RSDK-5391): remove this case
			// If we didn't pass a Configuration, but we do have a Position, then get the geometries at the zero state and
			// transform them to the Position
			internal, err := state.Frame.Geometries(make([]referenceframe.Input, len(state.Frame.DoF())))
			if err != nil {
				return false
			}
			movedGeoms := internal.Geometries()
			for _, geom := range movedGeoms {
				internalGeoms = append(internalGeoms, geom.Transform(state.Position))
			}
		default:
			return false
		}

		cg, err := newCollisionGraph(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
		if err != nil {
			return false
		}
		return len(cg.collisions(collisionBufferMM)) == 0
	}
	return constraint, nil
}

// NewAbsoluteLinearInterpolatingConstraint provides a Constraint whose valid manifold allows a specified amount of deviation from the
// shortest straight-line path between the start and the goal. linTol is the allowed linear deviation in mm, orientTol is the allowed
// orientation deviation measured by norm of the R3AA orientation difference to the slerp path between start/goal orientations.
func NewAbsoluteLinearInterpolatingConstraint(from, to spatial.Pose, linTol, orientTol float64) (StateConstraint, ik.StateMetric) {
	orientConstraint, orientMetric := NewSlerpOrientationConstraint(from, to, orientTol)
	lineConstraint, lineMetric := NewLineConstraint(from.Point(), to.Point(), linTol)
	interpMetric := ik.CombineMetrics(orientMetric, lineMetric)

	f := func(state *ik.State) bool {
		return orientConstraint(state) && lineConstraint(state)
	}
	return f, interpMetric
}

// NewProportionalLinearInterpolatingConstraint will provide the same metric and constraint as NewAbsoluteLinearInterpolatingConstraint,
// except that allowable linear and orientation deviation is scaled based on the distance from start to goal.
func NewProportionalLinearInterpolatingConstraint(from, to spatial.Pose, epsilon float64) (StateConstraint, ik.StateMetric) {
	orientTol := epsilon * ik.OrientDist(from.Orientation(), to.Orientation())
	linTol := epsilon * from.Point().Distance(to.Point())

	return NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
}

// NewSlerpOrientationConstraint will measure the orientation difference between the orientation of two poses, and return a constraint that
// returns whether a given orientation is within a given tolerance distance of the shortest segment between the two orientations, as
// well as a metric which returns the distance to that valid region.
func NewSlerpOrientationConstraint(start, goal spatial.Pose, tolerance float64) (StateConstraint, ik.StateMetric) {
	origDist := math.Max(ik.OrientDist(start.Orientation(), goal.Orientation()), defaultEpsilon)

	gradFunc := func(state *ik.State) float64 {
		sDist := ik.OrientDist(start.Orientation(), state.Position.Orientation())
		gDist := 0.

		// If origDist is less than or equal to defaultEpsilon, then the starting and ending orientations are the same and we do not need
		// to compute the distance to the ending orientation
		if origDist > defaultEpsilon {
			gDist = ik.OrientDist(goal.Orientation(), state.Position.Orientation())
		}
		return (sDist + gDist) - origDist
	}

	validFunc := func(state *ik.State) bool {
		err := resolveStatesToPositions(state)
		if err != nil {
			return false
		}
		return gradFunc(state) < tolerance
	}

	return validFunc, gradFunc
}

// NewPlaneConstraint is used to define a constraint space for a plane, and will return 1) a constraint
// function which will determine whether a point is on the plane and in a valid orientation, and 2) a distance function
// which will bring a pose into the valid constraint space. The plane normal is assumed to point towards the valid area.
// angle refers to the maximum unit sphere segment length deviation from the ov
// epsilon refers to the closeness to the plane necessary to be a valid pose.
func NewPlaneConstraint(pNorm, pt r3.Vector, writingAngle, epsilon float64) (StateConstraint, ik.StateMetric) {
	// get the constant value for the plane
	pConst := -pt.Dot(pNorm)

	// invert the normal to get the valid AOA OV
	ov := &spatial.OrientationVector{OX: -pNorm.X, OY: -pNorm.Y, OZ: -pNorm.Z}
	ov.Normalize()

	dFunc := ik.OrientDistToRegion(ov, writingAngle)

	// distance from plane to point
	planeDist := func(pt r3.Vector) float64 {
		return math.Abs(pNorm.Dot(pt) + pConst)
	}

	// TODO: do we need to care about trajectory here? Probably, but not yet implemented
	gradFunc := func(state *ik.State) float64 {
		pDist := planeDist(state.Position.Point())
		oDist := dFunc(state.Position.Orientation())
		return pDist*pDist + oDist*oDist
	}

	validFunc := func(state *ik.State) bool {
		err := resolveStatesToPositions(state)
		if err != nil {
			return false
		}
		return gradFunc(state) < epsilon*epsilon
	}

	return validFunc, gradFunc
}

// NewLineConstraint is used to define a constraint space for a line, and will return 1) a constraint
// function which will determine whether a point is on the line, and 2) a distance function
// which will bring a pose into the valid constraint space.
// tolerance refers to the closeness to the line necessary to be a valid pose in mm.
func NewLineConstraint(pt1, pt2 r3.Vector, tolerance float64) (StateConstraint, ik.StateMetric) {
	if pt1.Distance(pt2) < defaultEpsilon {
		tolerance = defaultEpsilon
	}

	gradFunc := func(state *ik.State) float64 {
		return math.Max(spatial.DistToLineSegment(pt1, pt2, state.Position.Point())-tolerance, 0)
	}

	validFunc := func(state *ik.State) bool {
		err := resolveStatesToPositions(state)
		if err != nil {
			return false
		}
		return gradFunc(state) == 0
	}

	return validFunc, gradFunc
}

// NewOctreeCollisionConstraint takes an octree and will return a constraint that checks whether any of the geometries in the solver frame
// intersect with points in the octree. Threshold sets the confidence level required for a point to be considered, and buffer is the
// distance to a point that is considered a collision in mm.
func NewOctreeCollisionConstraint(octree *pointcloud.BasicOctree, threshold int, buffer, collisionBufferMM float64) StateConstraint {
	constraint := func(state *ik.State) bool {
		geometries, err := state.Frame.Geometries(state.Configuration)
		if err != nil && geometries == nil {
			return false
		}

		for _, geom := range geometries.Geometries() {
			collides, err := octree.CollidesWithGeometry(geom, threshold, buffer, collisionBufferMM)
			if err != nil || collides {
				return false
			}
		}
		return true
	}
	return constraint
}

// NewBoundingRegionConstraint will determine if the given list of robot geometries are in collision with the
// given list of bounding regions.
func NewBoundingRegionConstraint(robotGeoms, boundingRegions []spatial.Geometry, collisionBufferMM float64) StateConstraint {
	return func(state *ik.State) bool {
		var internalGeoms []spatial.Geometry
		switch {
		case state.Configuration != nil:
			internal, err := state.Frame.Geometries(state.Configuration)
			if err != nil {
				return false
			}
			internalGeoms = internal.Geometries()
		case state.Position != nil:
			// TODO(RSDK-5391): remove this case
			// If we didn't pass a Configuration, but we do have a Position, then get the geometries at the zero state and
			// transform them to the Position
			internal, err := state.Frame.Geometries(make([]referenceframe.Input, len(state.Frame.DoF())))
			if err != nil {
				return false
			}
			movedGeoms := internal.Geometries()
			for _, geom := range movedGeoms {
				internalGeoms = append(internalGeoms, geom.Transform(state.Position))
			}
		default:
			internalGeoms = robotGeoms
		}
		cg, err := newCollisionGraph(internalGeoms, boundingRegions, nil, true, collisionBufferMM)
		if err != nil {
			return false
		}
		return len(cg.collisions(collisionBufferMM)) != 0
	}
}

// LinearConstraint specifies that the component being moved should move linearly relative to its goal.
// It does not constrain the motion of components other than the `component_name` specified in motion.Move.
type LinearConstraint struct {
	LineToleranceMm          float64 // Max linear deviation from straight-line between start and goal, in mm.
	OrientationToleranceDegs float64
	MovingFrame string
	GoalFrame string
}

// OrientationConstraint specifies that the component being moved will not deviate its orientation beyond some threshold relative
// to the goal. It does not constrain the motion of components other than the `component_name` specified in motion.Move.
type OrientationConstraint struct {
	OrientationToleranceDegs float64
	MovingFrame string
	GoalFrame string
}

// CollisionSpecificationAllowedFrameCollisions is used to define frames that are allowed to collide.
type CollisionSpecificationAllowedFrameCollisions struct {
	Frame1, Frame2 string
}

// CollisionSpecification is used to selectively apply obstacle avoidance to specific parts of the robot.
type CollisionSpecification struct {
	// Pairs of frame which should be allowed to collide with one another
	Allows []CollisionSpecificationAllowedFrameCollisions
}

// Constraints is a struct to store the constraints imposed upon a robot
// It serves as a convenenient RDK wrapper for the protobuf object.
type Constraints struct {
	LinearConstraint       []LinearConstraint
	OrientationConstraint  []OrientationConstraint
	CollisionSpecification []CollisionSpecification
}

// NewEmptyConstraints creates a new, empty Constraints object.
func NewEmptyConstraints() *Constraints {
	return &Constraints{
		LinearConstraint:       make([]LinearConstraint, 0),
		OrientationConstraint:  make([]OrientationConstraint, 0),
		CollisionSpecification: make([]CollisionSpecification, 0),
	}
}

// NewConstraints initializes a Constraints object with user-defined LinearConstraint, OrientationConstraint, and CollisionSpecification.
func NewConstraints(
	linConstraints []LinearConstraint,
	orientConstraints []OrientationConstraint,
	collSpecifications []CollisionSpecification,
) *Constraints {
	return &Constraints{
		LinearConstraint:       linConstraints,
		OrientationConstraint:  orientConstraints,
		CollisionSpecification: collSpecifications,
	}
}

// ConstraintsFromProtobuf converts a protobuf object to a Constraints object.
func ConstraintsFromProtobuf(pbConstraint *motionpb.Constraints) *Constraints {
	if pbConstraint == nil {
		return NewEmptyConstraints()
	}

	// iterate through all motionpb.LinearConstraint and convert to RDK form
	linConstraintFromProto := func(linConstraints []*motionpb.LinearConstraint) []LinearConstraint {
		toRet := make([]LinearConstraint, 0, len(linConstraints))
		for _, linConstraint := range linConstraints {
			linTol := 0.
			if linConstraint.LineToleranceMm != nil {
				linTol = float64(*linConstraint.LineToleranceMm)
			}
			orientTol := 0.
			if linConstraint.OrientationToleranceDegs != nil {
				orientTol = float64(*linConstraint.OrientationToleranceDegs)
			}
			toRet = append(toRet, LinearConstraint{
				LineToleranceMm:          linTol,
				OrientationToleranceDegs: orientTol,
			})
		}
		return toRet
	}

	// iterate through all motionpb.OrientationConstraint and convert to RDK form
	orientConstraintFromProto := func(orientConstraints []*motionpb.OrientationConstraint) []OrientationConstraint {
		toRet := make([]OrientationConstraint, 0, len(orientConstraints))
		for _, orientConstraint := range orientConstraints {
			toRet = append(toRet, OrientationConstraint{
				OrientationToleranceDegs: float64(*orientConstraint.OrientationToleranceDegs),
			})
		}
		return toRet
	}

	// iterate through all motionpb.CollisionSpecification and convert to RDK form
	collSpecFromProto := func(collSpecs []*motionpb.CollisionSpecification) []CollisionSpecification {
		toRet := make([]CollisionSpecification, 0, len(collSpecs))
		for _, collSpec := range collSpecs {
			allowedFrameCollisions := make([]CollisionSpecificationAllowedFrameCollisions, 0)
			for _, collSpecAllowedFrame := range collSpec.Allows {
				allowedFrameCollisions = append(allowedFrameCollisions, CollisionSpecificationAllowedFrameCollisions{
					Frame1: collSpecAllowedFrame.Frame1,
					Frame2: collSpecAllowedFrame.Frame2,
				})
			}
			toRet = append(toRet, CollisionSpecification{
				Allows: allowedFrameCollisions,
			})
		}
		return toRet
	}

	return NewConstraints(
		linConstraintFromProto(pbConstraint.LinearConstraint),
		orientConstraintFromProto(pbConstraint.OrientationConstraint),
		collSpecFromProto(pbConstraint.CollisionSpecification),
	)
}

// ToProtobuf takes an existing Constraints object and converts it to a protobuf.
func (c *Constraints) ToProtobuf() *motionpb.Constraints {
	if c == nil {
		return nil
	}
	// convert LinearConstraint to motionpb.LinearConstraint
	convertLinConstraintToProto := func(linConstraints []LinearConstraint) []*motionpb.LinearConstraint {
		toRet := make([]*motionpb.LinearConstraint, 0)
		for _, linConstraint := range linConstraints {
			lineTolerance := float32(linConstraint.LineToleranceMm)
			orientationTolerance := float32(linConstraint.OrientationToleranceDegs)
			toRet = append(toRet, &motionpb.LinearConstraint{
				LineToleranceMm:          &lineTolerance,
				OrientationToleranceDegs: &orientationTolerance,
			})
		}
		return toRet
	}

	// convert OrientationConstraint to motionpb.OrientationConstraint
	convertOrientConstraintToProto := func(orientConstraints []OrientationConstraint) []*motionpb.OrientationConstraint {
		toRet := make([]*motionpb.OrientationConstraint, 0)
		for _, orientConstraint := range orientConstraints {
			orientationTolerance := float32(orientConstraint.OrientationToleranceDegs)
			toRet = append(toRet, &motionpb.OrientationConstraint{
				OrientationToleranceDegs: &orientationTolerance,
			})
		}
		return toRet
	}

	// convert CollisionSpecifications to motionpb.CollisionSpecification
	convertCollSpecToProto := func(collSpecs []CollisionSpecification) []*motionpb.CollisionSpecification {
		toRet := make([]*motionpb.CollisionSpecification, 0)
		for _, collSpec := range collSpecs {
			allowedFrameCollisions := make([]*motionpb.CollisionSpecification_AllowedFrameCollisions, 0)
			for _, collSpecAllowedFrame := range collSpec.Allows {
				allowedFrameCollisions = append(allowedFrameCollisions, &motionpb.CollisionSpecification_AllowedFrameCollisions{
					Frame1: collSpecAllowedFrame.Frame1,
					Frame2: collSpecAllowedFrame.Frame2,
				})
			}
			toRet = append(toRet, &motionpb.CollisionSpecification{
				Allows: allowedFrameCollisions,
			})
		}
		return toRet
	}

	return &motionpb.Constraints{
		LinearConstraint:       convertLinConstraintToProto(c.LinearConstraint),
		OrientationConstraint:  convertOrientConstraintToProto(c.OrientationConstraint),
		CollisionSpecification: convertCollSpecToProto(c.CollisionSpecification),
	}
}

// AddLinearConstraint appends a LinearConstraint to a Constraints object.
func (c *Constraints) AddLinearConstraint(linConstraint LinearConstraint) {
	c.LinearConstraint = append(c.LinearConstraint, linConstraint)
}

// GetLinearConstraint checks if the Constraints object is nil and if not then returns its LinearConstraint field.
func (c *Constraints) GetLinearConstraint() []LinearConstraint {
	if c != nil {
		return c.LinearConstraint
	}
	return nil
}

// AddOrientationConstraint appends a OrientationConstraint to a Constraints object.
func (c *Constraints) AddOrientationConstraint(orientConstraint OrientationConstraint) {
	c.OrientationConstraint = append(c.OrientationConstraint, orientConstraint)
}

// GetOrientationConstraint checks if the Constraints object is nil and if not then returns its OrientationConstraint field.
func (c *Constraints) GetOrientationConstraint() []OrientationConstraint {
	if c != nil {
		return c.OrientationConstraint
	}
	return nil
}

// AddCollisionSpecification appends a CollisionSpecification to a Constraints object.
func (c *Constraints) AddCollisionSpecification(collConstraint CollisionSpecification) {
	c.CollisionSpecification = append(c.CollisionSpecification, collConstraint)
}

// GetCollisionSpecification checks if the Constraints object is nil and if not then returns its CollisionSpecification field.
func (c *Constraints) GetCollisionSpecification() []CollisionSpecification {
	if c != nil {
		return c.CollisionSpecification
	}
	return nil
}

// CheckStateConstraintsAcrossSegmentFS will interpolate the given input from the StartInput to the EndInput, and ensure that all intermediate
// states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will return `true, nil`.
// If any constraints fail, this will return false, and an SegmentFS representing the valid portion of the segment, if any. If no
// part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintHandler) CheckStateConstraintsAcrossSegmentFS(ci *ik.SegmentFS, resolution float64) (bool, *ik.SegmentFS) {
	interpolatedConfigurations, err := interpolateSegmentFS(ci, resolution)
	if err != nil {
		return false, nil
	}
	var lastGood map[string][]referenceframe.Input
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &ik.StateFS{FS: ci.FS, Configuration: interpConfig}
		pass, _ := c.CheckStateFSConstraints(interpC)
		if !pass {
			if i == 0 {
				// fail on start pos
				return false, nil
			}
			return false, &ik.SegmentFS{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, FS: ci.FS}
		}
		lastGood = interpC.Configuration
	}

	return true, nil
}

// CheckSegmentAndStateValidityFS will check a segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintHandler) CheckSegmentAndStateValidityFS(segment *ik.SegmentFS, resolution float64) (bool, *ik.SegmentFS) {
	valid, subSegment := c.CheckStateConstraintsAcrossSegmentFS(segment, resolution)
	if !valid {
		if subSegment != nil {
			subSegmentValid, _ := c.CheckSegmentFSConstraints(subSegment)
			if subSegmentValid {
				return false, subSegment
			}
		}
		return false, nil
	}
	// all states are valid
	valid, _ = c.CheckSegmentFSConstraints(segment)
	return valid, nil
}
