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

func createAllCollisionConstraints(
	movingRobotGeometries, staticRobotGeometries, worldGeometries, boundingRegions []spatial.Geometry,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) (map[string]StateFSConstraint, map[string]StateConstraint, error) {
	constraintFSMap := map[string]StateFSConstraint{}
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
						return nil, nil, err
					}
				}
				for _, collision := range zeroCG.collisions(collisionBufferMM) {
					if collision.name1 == octree.Label() {
						return nil, nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name2)
					} else if collision.name2 == octree.Label() {
						return nil, nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name1)
					}
				}
			}
		}

		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraint, err := NewCollisionConstraint(movingRobotGeometries, worldGeometries, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraintFS, err := NewCollisionConstraintFS(movingRobotGeometries, worldGeometries, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		// TODO: TPspace currently still uses the non-FS constraint, this should be removed once TPspace is fully migrated to frame systems
		constraintMap[defaultObstacleConstraintDesc] = obstacleConstraint
		constraintFSMap[defaultObstacleConstraintDesc] = obstacleConstraintFS
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
			return nil, nil, err
		}
		robotConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			staticRobotGeometries,
			allowedCollisions,
			false,
			collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		constraintMap[defaultRobotCollisionConstraintDesc] = robotConstraint
		constraintFSMap[defaultRobotCollisionConstraintDesc] = robotConstraintFS
	}

	// create constraint to keep moving geometries from hitting themselves
	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraint, err := NewCollisionConstraint(movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		constraintMap[defaultSelfCollisionConstraintDesc] = selfCollisionConstraint
		selfCollisionConstraintFS, err := NewCollisionConstraintFS(movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		constraintFSMap[defaultSelfCollisionConstraintDesc] = selfCollisionConstraintFS
	}
	return constraintFSMap, constraintMap, nil
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

// NewCollisionConstraintFS is the most general method to create a collision constraint for a frame system,
// which will be violated if geometries constituting the given frame ever come into collision with obstacle geometries
// outside of the collisions present for the observationInput. Collisions specified as collisionSpecifications will also be ignored.
// If reportDistances is false, this check will be done as fast as possible, if true maximum information will be available for debugging.
func NewCollisionConstraintFS(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (StateFSConstraint, error) {
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, collisionBufferMM)
	if err != nil {
		return nil, err
	}
	movingMap := map[string]spatial.Geometry{}
	for _, geom := range moving {
		movingMap[geom.Label()] = geom
	}

	// create constraint from reference collision graph
	constraint := func(state *ik.StateFS) bool {
		// Use FrameSystemGeometries to get all geometries in the frame system
		internalGeometries, err := referenceframe.FrameSystemGeometries(state.FS, state.Configuration)
		if err != nil {
			return false
		}

		// We only want to compare *moving* geometries, so we filter what we get from the framesystem against what we were passed.
		var internalGeoms []spatial.Geometry
		for _, geosInFrame := range internalGeometries {
			if len(geosInFrame.Geometries()) > 0 {
				if _, ok := movingMap[geosInFrame.Geometries()[0].Label()]; ok {
					internalGeoms = append(internalGeoms, geosInFrame.Geometries()...)
				}
			}
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
	// Account for float error
	if linTol < defaultEpsilon {
		linTol = defaultEpsilon
	}
	if orientTol < defaultEpsilon {
		orientTol = defaultEpsilon
	}

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
func NewProportionalLinearInterpolatingConstraint(
	from, to spatial.Pose,
	linEpsilon, orientEpsilon float64,
) (StateConstraint, ik.StateMetric) {
	orientTol := orientEpsilon * ik.OrientDist(from.Orientation(), to.Orientation())
	linTol := linEpsilon * from.Point().Distance(to.Point())

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

// NewOctreeCollisionConstraint takes an octree and will return a constraint that checks whether any geometries
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

// LinearConstraint specifies that the components being moved should move linearly relative to their goals.
type LinearConstraint struct {
	LineToleranceMm          float64 // Max linear deviation from straight-line between start and goal, in mm.
	OrientationToleranceDegs float64
}

// PseudolinearConstraint specifies that the component being moved should not deviate from the straight-line path to their goal by
// more than a factor proportional to the distance from start to goal.
// For example, if a component is moving 100mm, then a LineToleranceFactor of 1.0 means that the component will remain within a 100mm
// radius of the straight-line start-goal path.
type PseudolinearConstraint struct {
	LineToleranceFactor        float64
	OrientationToleranceFactor float64
}

// OrientationConstraint specifies that the components being moved will not deviate orientation beyond some threshold.
type OrientationConstraint struct {
	OrientationToleranceDegs float64
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
	PseudolinearConstraint []PseudolinearConstraint
	OrientationConstraint  []OrientationConstraint
	CollisionSpecification []CollisionSpecification
}

// NewEmptyConstraints creates a new, empty Constraints object.
func NewEmptyConstraints() *Constraints {
	return &Constraints{
		LinearConstraint:       make([]LinearConstraint, 0),
		PseudolinearConstraint: make([]PseudolinearConstraint, 0),
		OrientationConstraint:  make([]OrientationConstraint, 0),
		CollisionSpecification: make([]CollisionSpecification, 0),
	}
}

// NewConstraints initializes a Constraints object with user-defined LinearConstraint, OrientationConstraint, and CollisionSpecification.
func NewConstraints(
	linConstraints []LinearConstraint,
	pseudoConstraints []PseudolinearConstraint,
	orientConstraints []OrientationConstraint,
	collSpecifications []CollisionSpecification,
) *Constraints {
	return &Constraints{
		LinearConstraint:       linConstraints,
		PseudolinearConstraint: pseudoConstraints,
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
		[]PseudolinearConstraint{},
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

// AddPseudolinearConstraint appends a PseudolinearConstraint to a Constraints object.
func (c *Constraints) AddPseudolinearConstraint(plinConstraint PseudolinearConstraint) {
	c.PseudolinearConstraint = append(c.PseudolinearConstraint, plinConstraint)
}

// GetPseudolinearConstraint checks if the Constraints object is nil and if not then returns its PseudolinearConstraint field.
func (c *Constraints) GetPseudolinearConstraint() []PseudolinearConstraint {
	if c != nil {
		return c.PseudolinearConstraint
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

type fsPathConstraint struct {
	metricMap     map[string]ik.StateMetric
	constraintMap map[string]StateConstraint
	goalMap       referenceframe.FrameSystemPoses
	fs            referenceframe.FrameSystem
}

func (fpc *fsPathConstraint) constraint(state *ik.StateFS) bool {
	for frame, goal := range fpc.goalMap {
		if constraint, ok := fpc.constraintMap[frame]; ok {
			currPose, err := fpc.fs.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				return false
			}
			pass := constraint(&ik.State{
				Configuration: state.Configuration[frame],
				Position:      currPose.(*referenceframe.PoseInFrame).Pose(),
				Frame:         fpc.fs.Frame(frame),
			})
			if !pass {
				return false
			}
		}
	}
	return true
}

func (fpc *fsPathConstraint) metric(state *ik.StateFS) float64 {
	score := 0.
	for frame, goal := range fpc.goalMap {
		if metric, ok := fpc.metricMap[frame]; ok {
			currPose, err := fpc.fs.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				score = math.Inf(1)
				break
			}
			score += metric(&ik.State{
				Configuration: state.Configuration[frame],
				Position:      currPose.(*referenceframe.PoseInFrame).Pose(),
				Frame:         fpc.fs.Frame(frame),
			})
		}
	}
	return score
}

func newFsPathConstraintSeparatedLinOrientTol(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	constructor func(spatial.Pose, spatial.Pose, float64, float64) (StateConstraint, ik.StateMetric),
	linTol, orientTol float64,
) (*fsPathConstraint, error) {
	metricMap := map[string]ik.StateMetric{}
	constraintMap := map[string]StateConstraint{}

	for frame, goal := range to {
		startPiF, ok := from[frame]
		if !ok {
			startPiFTf, err := fs.Transform(startCfg, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				return nil, err
			}
			startPiF = startPiFTf.(*referenceframe.PoseInFrame)
		}
		constraint, metric := constructor(startPiF.Pose(), goal.Pose(), linTol, orientTol)

		metricMap[frame] = metric
		constraintMap[frame] = constraint
	}
	return &fsPathConstraint{
		metricMap:     metricMap,
		constraintMap: constraintMap,
		goalMap:       to,
		fs:            fs,
	}, nil
}

func newFsPathConstraintTol(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	constructor func(spatial.Pose, spatial.Pose, float64) (StateConstraint, ik.StateMetric),
	tolerance float64,
) (*fsPathConstraint, error) {
	metricMap := map[string]ik.StateMetric{}
	constraintMap := map[string]StateConstraint{}

	for frame, goal := range to {
		startPiF, ok := from[frame]
		if !ok {
			startPiFTf, err := fs.Transform(startCfg, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				return nil, err
			}
			startPiF = startPiFTf.(*referenceframe.PoseInFrame)
		}
		constraint, metric := constructor(startPiF.Pose(), goal.Pose(), tolerance)

		metricMap[frame] = metric
		constraintMap[frame] = constraint
	}
	return &fsPathConstraint{
		metricMap:     metricMap,
		constraintMap: constraintMap,
		goalMap:       to,
		fs:            fs,
	}, nil
}

// CreateSlerpOrientationConstraintFS will measure the orientation difference between the orientation of two poses across a frame system,
// and return a constraint that returns whether given orientations are within a given tolerance distance of the shortest segment between
// their respective orientations, as well as a metric which returns the distance to that valid region.
func CreateSlerpOrientationConstraintFS(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	tolerance float64,
) (StateFSConstraint, ik.StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintTol(fs, startCfg, from, to, NewSlerpOrientationConstraint, tolerance)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}

// CreateLineConstraintFS will measure the linear distance between the positions of two poses across a frame system,
// and return a constraint that checks whether given positions are within a specified tolerance distance of the shortest
// line segment between their respective positions, as well as a metric which returns the distance to that valid region.
func CreateLineConstraintFS(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	tolerance float64,
) (StateFSConstraint, ik.StateFSMetric, error) {
	// Need to define a constructor here since NewLineConstraint takes r3.Vectors, not poses
	constructor := func(fromPose, toPose spatial.Pose, tolerance float64) (StateConstraint, ik.StateMetric) {
		return NewLineConstraint(fromPose.Point(), toPose.Point(), tolerance)
	}
	constraintInternal, err := newFsPathConstraintTol(fs, startCfg, from, to, constructor, tolerance)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}

// CreateAbsoluteLinearInterpolatingConstraintFS provides a Constraint whose valid manifold allows a specified amount of deviation from the
// shortest straight-line path between the start and the goal. linTol is the allowed linear deviation in mm, orientTol is the allowed
// orientation deviation measured by norm of the R3AA orientation difference to the slerp path between start/goal orientations.
func CreateAbsoluteLinearInterpolatingConstraintFS(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	linTol, orientTol float64,
) (StateFSConstraint, ik.StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintSeparatedLinOrientTol(
		fs,
		startCfg,
		from,
		to,
		NewAbsoluteLinearInterpolatingConstraint,
		linTol,
		orientTol,
	)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}

// CreateProportionalLinearInterpolatingConstraintFS will provide the same metric and constraint as
// CreateAbsoluteLinearInterpolatingConstraintFS, except that allowable linear and orientation deviation is scaled based on the distance
// from start to goal.
func CreateProportionalLinearInterpolatingConstraintFS(
	fs referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	linTol, orientTol float64,
) (StateFSConstraint, ik.StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintSeparatedLinOrientTol(
		fs,
		startCfg,
		from,
		to,
		NewProportionalLinearInterpolatingConstraint,
		linTol,
		orientTol,
	)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}
