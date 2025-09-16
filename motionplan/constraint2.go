package motionplan

import (
	"errors"
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// short descriptions of constraints used in error messages.
const (
	linearConstraintDescription      = "linear constraint"
	orientationConstraintDescription = "orientation constraint"
	planarConstraintDescription      = "planar constraint"

	// various collision constraints that have different names in order to be unique keys in maps of constraints that are created.
	boundingRegionConstraintDescription = "bounding region constraint"
	obstacleConstraintDescription       = "obstacle constraint"
	selfCollisionConstraintDescription  = "self-collision constraint"
	robotCollisionConstraintDescription = "robot constraint" // collision between a moving robot component and one that is stationary

	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001

	// allowable deviation from slerp between start/goal orientations, unit is the number of degrees of rotation away from the most direct
	// arc from start orientation to goal orientation.
	defaultOrientationDeviation = 2.0

	defaultConstraintName = "unnamed constraint"

	// max linear deviation from straight-line between start and goal, in mm.
	defaultLinearDeviation = 0.1

	// allowable linear and orientation deviation from direct interpolation path, as a proportion of the linear and orientation distances
	// between the start and goal.
	defaultPseudolinearTolerance = 0.8

	defaultCollisionBufferMM = 1e-8
)

var errInvalidConstraint = errors.New("invalid constraint input")

// Given a constraint input with only frames and input positions, calculates the corresponding poses as needed.
func resolveSegmentsToPositions(segment *Segment) error {
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

// SegmentFSConstraint tests whether a transition from a starting robot configuration to an ending robot configuration is valid.
// If the returned error is nil, the constraint is satisfied and the segment is valid.
type SegmentFSConstraint func(*SegmentFS) error

// SegmentConstraint tests whether a transition from a starting robot configuration to an ending robot configuration is valid.
// If the returned error is nil, the constraint is satisfied and the segment is valid.
type SegmentConstraint func(*Segment) error

// StateFSConstraint tests whether a given robot configuration is valid
// If the returned error is nil, the constraint is satisfied and the state is valid.
type StateFSConstraint func(*StateFS) error

// StateConstraint tests whether a given robot configuration is valid
// If the returned error is nil, the constraint is satisfied and the state is valid.
type StateConstraint func(*State) error

// CreateAllCollisionConstraints -.
func CreateAllCollisionConstraints(
	movingRobotGeometries, staticRobotGeometries, worldGeometries, boundingRegions []spatial.Geometry,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) (map[string]StateFSConstraint, map[string]StateConstraint, error) {
	constraintFSMap := map[string]StateFSConstraint{}
	constraintMap := map[string]StateConstraint{}

	if len(worldGeometries) > 0 {
		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraint, err := NewCollisionConstraint(
			movingRobotGeometries,
			worldGeometries,
			allowedCollisions,
			false,
			collisionBufferMM,
		)
		if err != nil {
			return nil, nil, err
		}
		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			worldGeometries,
			allowedCollisions,
			false,
			collisionBufferMM,
		)
		if err != nil {
			return nil, nil, err
		}
		// TODO: TPspace currently still uses the non-FS constraint, this should be removed once TPspace is fully migrated to frame systems
		constraintMap[obstacleConstraintDescription] = obstacleConstraint
		constraintFSMap[obstacleConstraintDescription] = obstacleConstraintFS
	}

	if len(boundingRegions) > 0 {
		// create constraint to keep moving geometries within the defined bounding regions
		interactionSpaceConstraint := NewBoundingRegionConstraint(movingRobotGeometries, boundingRegions, collisionBufferMM)
		constraintMap[boundingRegionConstraintDescription] = interactionSpaceConstraint
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
		constraintMap[robotCollisionConstraintDescription] = robotConstraint
		constraintFSMap[robotCollisionConstraintDescription] = robotConstraintFS
	}

	// create constraint to keep moving geometries from hitting themselves
	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraint, err := NewCollisionConstraint(movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		constraintMap[selfCollisionConstraintDescription] = selfCollisionConstraint
		selfCollisionConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, nil, err
		}
		constraintFSMap[selfCollisionConstraintDescription] = selfCollisionConstraintFS
	}
	return constraintFSMap, constraintMap, nil
}

func setupZeroCG(
	moving, static []spatial.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (*collisionGraph, error) {
	// create the reference collisionGraph
	zeroCG, err := newCollisionGraph(moving, static, nil, reportDistances, collisionBufferMM)
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
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, true, collisionBufferMM)
	if err != nil {
		return nil, err
	}

	// create constraint from reference collision graph
	constraint := func(state *State) error {
		var internalGeoms []spatial.Geometry
		switch {
		case state.Configuration != nil:
			internal, err := state.Frame.Geometries(state.Configuration)
			if err != nil {
				return err
			}
			internalGeoms = internal.Geometries()
		case state.Position != nil:
			// TODO(RSDK-5391): remove this case
			// If we didn't pass a Configuration, but we do have a Position, then get the geometries at the zero state and
			// transform them to the Position
			internal, err := state.Frame.Geometries(make([]referenceframe.Input, len(state.Frame.DoF())))
			if err != nil {
				return err
			}
			movedGeoms := internal.Geometries()
			for _, geom := range movedGeoms {
				internalGeoms = append(internalGeoms, geom.Transform(state.Position))
			}
		default:
			return errors.New("need either a Position or Configuration to be set for a State")
		}

		return collisionCheckFinish(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	}
	return constraint, nil
}

func collisionCheckFinish(internalGeoms, static []spatial.Geometry, zeroCG *collisionGraph,
	reportDistances bool, collisionBufferMM float64,
) error {
	cg, err := newCollisionGraph(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	if err != nil {
		return err
	}
	cs := cg.collisions(collisionBufferMM)
	if len(cs) != 0 {
		// we could choose to amalgamate all the collisions into one error but its probably saner not to and choose just the first
		return fmt.Errorf("violation between %s and %s geometries (tolal collisions: %d)", cs[0].name1, cs[0].name2, len(cs))
	}
	return nil
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
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, true, collisionBufferMM)
	if err != nil {
		return nil, err
	}

	movingMap := map[string]spatial.Geometry{}
	for _, geom := range moving {
		movingMap[geom.Label()] = geom
	}

	// create constraint from reference collision graph
	constraint := func(state *StateFS) error {
		// Use FrameSystemGeometries to get all geometries in the frame system
		internalGeometries, err := referenceframe.FrameSystemGeometries(state.FS, state.Configuration)
		if err != nil {
			return err
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

		return collisionCheckFinish(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	}
	return constraint, nil
}

// newAbsoluteLinearInterpolatingConstraint provides a Constraint whose valid manifold allows a specified amount of deviation from the
// shortest straight-line path between the start and the goal. linTol is the allowed linear deviation in mm, orientTol is the allowed
// orientation deviation measured by norm of the R3AA orientation difference to the slerp path between start/goal orientations.
func newAbsoluteLinearInterpolatingConstraint(from, to spatial.Pose, linTol, orientTol float64) (StateConstraint, StateMetric) {
	// Account for float error
	if linTol < defaultEpsilon {
		linTol = defaultEpsilon
	}
	if orientTol < defaultEpsilon {
		orientTol = defaultEpsilon
	}

	orientConstraint, orientMetric := newSlerpOrientationConstraint(from, to, orientTol)
	lineConstraint, lineMetric := NewLineConstraint(from.Point(), to.Point(), linTol)
	interpMetric := CombineMetrics(orientMetric, lineMetric)

	f := func(state *State) error {
		return errors.Join(orientConstraint(state), lineConstraint(state))
	}
	return f, interpMetric
}

// newProportionalLinearInterpolatingConstraint will provide the same metric and constraint as newAbsoluteLinearInterpolatingConstraint,
// except that allowable linear and orientation deviation is scaled based on the distance from start to goal.
func newProportionalLinearInterpolatingConstraint(
	from, to spatial.Pose,
	linEpsilon, orientEpsilon float64,
) (StateConstraint, StateMetric) {
	orientTol := orientEpsilon * OrientDist(from.Orientation(), to.Orientation())
	linTol := linEpsilon * from.Point().Distance(to.Point())

	return newAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
}

// newSlerpOrientationConstraint will measure the orientation difference between the orientation of two poses, and return a constraint that
// returns whether a given orientation is within a given tolerance distance of the shortest segment between the two orientations, as
// well as a metric which returns the distance to that valid region.
func newSlerpOrientationConstraint(start, goal spatial.Pose, tolerance float64) (StateConstraint, StateMetric) {
	origDist := math.Max(OrientDist(start.Orientation(), goal.Orientation()), defaultEpsilon)

	gradFunc := func(state *State) float64 {
		sDist := OrientDist(start.Orientation(), state.Position.Orientation())
		gDist := 0.

		// If origDist is less than or equal to defaultEpsilon, then the starting and ending orientations are the same and we do not need
		// to compute the distance to the ending orientation
		if origDist > defaultEpsilon {
			gDist = OrientDist(goal.Orientation(), state.Position.Orientation())
		}
		return (sDist + gDist) - origDist
	}

	validFunc := func(state *State) error {
		err := state.ResolveStateAndUpdatePositions()
		if err != nil {
			return err
		}
		if gradFunc(state) < tolerance {
			return nil
		}
		return errors.New(orientationConstraintDescription + " violated")
	}

	return validFunc, gradFunc
}

// NewLineConstraint is used to define a constraint space for a line, and will return 1) a constraint
// function which will determine whether a point is on the line, and 2) a distance function
// which will bring a pose into the valid constraint space.
// tolerance refers to the closeness to the line necessary to be a valid pose in mm.
func NewLineConstraint(pt1, pt2 r3.Vector, tolerance float64) (StateConstraint, StateMetric) {
	gradFunc := func(state *State) float64 {
		return math.Max(spatial.DistToLineSegment(pt1, pt2, state.Position.Point())-tolerance, 0)
	}

	validFunc := func(state *State) error {
		err := state.ResolveStateAndUpdatePositions()
		if err != nil {
			return err
		}
		if gradFunc(state) == 0 {
			return nil
		}
		return errors.New(linearConstraintDescription + " violated")
	}

	return validFunc, gradFunc
}

// NewBoundingRegionConstraint will determine if the given list of robot geometries are in collision with the
// given list of bounding regions.
func NewBoundingRegionConstraint(robotGeoms, boundingRegions []spatial.Geometry, collisionBufferMM float64) StateConstraint {
	return func(state *State) error {
		var internalGeoms []spatial.Geometry
		switch {
		case state.Configuration != nil:
			internal, err := state.Frame.Geometries(state.Configuration)
			if err != nil {
				return err
			}
			internalGeoms = internal.Geometries()
		case state.Position != nil:
			// If we didn't pass a Configuration, but we do have a Position, then get the geometries at the zero state and
			// transform them to the Position
			internal, err := state.Frame.Geometries(make([]referenceframe.Input, len(state.Frame.DoF())))
			if err != nil {
				return err
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
			return err
		}
		cs := cg.collisions(collisionBufferMM)
		if len(cs) == 0 {
			return errors.New("violation of bounding region constraint")
		}
		return nil
	}
}

type fsPathConstraint struct {
	metricMap     map[string]StateMetric
	constraintMap map[string]StateConstraint
	goalMap       referenceframe.FrameSystemPoses
	fs            *referenceframe.FrameSystem
}

func (fpc *fsPathConstraint) constraint(state *StateFS) error {
	for frame, goal := range fpc.goalMap {
		if constraint, ok := fpc.constraintMap[frame]; ok {
			currPose, err := fpc.fs.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				return err
			}
			if err := constraint(&State{
				Configuration: state.Configuration[frame],
				Position:      currPose.(*referenceframe.PoseInFrame).Pose(),
				Frame:         fpc.fs.Frame(frame),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (fpc *fsPathConstraint) metric(state *StateFS) float64 {
	score := 0.
	for frame, goal := range fpc.goalMap {
		if metric, ok := fpc.metricMap[frame]; ok {
			currPose, err := fpc.fs.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), goal.Parent())
			if err != nil {
				score = math.Inf(1)
				break
			}
			score += metric(&State{
				Configuration: state.Configuration[frame],
				Position:      currPose.(*referenceframe.PoseInFrame).Pose(),
				Frame:         fpc.fs.Frame(frame),
			})
		}
	}
	return score
}

func newFsPathConstraintSeparatedLinOrientTol(
	fs *referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	constructor func(spatial.Pose, spatial.Pose, float64, float64) (StateConstraint, StateMetric),
	linTol, orientTol float64,
) (*fsPathConstraint, error) {
	metricMap := map[string]StateMetric{}
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
	fs *referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	constructor func(spatial.Pose, spatial.Pose, float64) (StateConstraint, StateMetric),
	tolerance float64,
) (*fsPathConstraint, error) {
	metricMap := map[string]StateMetric{}
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
func createSlerpOrientationConstraintFS(
	fs *referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	tolerance float64,
) (StateFSConstraint, StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintTol(fs, startCfg, from, to, newSlerpOrientationConstraint, tolerance)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}

// CreateAbsoluteLinearInterpolatingConstraintFS provides a Constraint whose valid manifold allows a specified amount of deviation from the
// shortest straight-line path between the start and the goal. linTol is the allowed linear deviation in mm, orientTol is the allowed
// orientation deviation measured by norm of the R3AA orientation difference to the slerp path between start/goal orientations.
func createAbsoluteLinearInterpolatingConstraintFS(
	fs *referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	linTol, orientTol float64,
) (StateFSConstraint, StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintSeparatedLinOrientTol(
		fs,
		startCfg,
		from,
		to,
		newAbsoluteLinearInterpolatingConstraint,
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
func createProportionalLinearInterpolatingConstraintFS(
	fs *referenceframe.FrameSystem,
	startCfg referenceframe.FrameSystemInputs,
	from, to referenceframe.FrameSystemPoses,
	linTol, orientTol float64,
) (StateFSConstraint, StateFSMetric, error) {
	constraintInternal, err := newFsPathConstraintSeparatedLinOrientTol(
		fs,
		startCfg,
		from,
		to,
		newProportionalLinearInterpolatingConstraint,
		linTol,
		orientTol,
	)
	if err != nil {
		return nil, nil, err
	}
	return constraintInternal.constraint, constraintInternal.metric, nil
}
