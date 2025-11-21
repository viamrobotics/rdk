package motionplan

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
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

	defaultCollisionBufferMM = 1e-8
	defaultMinStepCount      = 2
)

// StateFSConstraint tests whether a given robot configuration is valid
// If the returned error is nil, the constraint is satisfied and the state is valid.
type StateFSConstraint func(*StateFS) error

// ConstraintChecker is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type ConstraintChecker struct {
	stateFSConstraints map[string]StateFSConstraint
}

// NewEmptyConstraintChecker - creates a ConstraintChecker with nothing.
func NewEmptyConstraintChecker() *ConstraintChecker {
	return &ConstraintChecker{}
}

// NewConstraintChecker - creates a ConstraintChecker with all the params.
func NewConstraintChecker(
	collisionBufferMM float64,
	constraints *Constraints,
	startPoses, goalPoses referenceframe.FrameSystemPoses,
	fs *referenceframe.FrameSystem,
	movingRobotGeometries, staticRobotGeometries []spatialmath.Geometry,
	seedMap *referenceframe.LinearInputs,
	worldState *referenceframe.WorldState,
) (*ConstraintChecker, error) {
	if constraints == nil {
		// Constraints may be nil, but if a motion profile is set in planningOpts
		// we need it to be a valid pointer to an empty struct.
		constraints = &Constraints{}
	}
	handler := NewEmptyConstraintChecker()

	frameSystemGeometries, err := referenceframe.FrameSystemGeometriesLinearInputs(fs, seedMap)
	if err != nil {
		return nil, err
	}

	obstaclesInFrame, err := worldState.ObstaclesInWorldFrame(fs, seedMap.ToFrameSystemInputs())
	if err != nil {
		return nil, err
	}
	worldGeometries := obstaclesInFrame.Geometries()

	frameNames := map[string]bool{}
	for _, fName := range fs.FrameNames() {
		frameNames[fName] = true
	}

	allowedCollisions, err := collisionSpecifications(
		constraints.CollisionSpecification,
		frameSystemGeometries,
		frameNames,
		worldState.ObstacleNames(),
	)
	if err != nil {
		return nil, err
	}

	// add collision constraints
	fsCollisionConstraints, err := CreateAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries,
		allowedCollisions,
		collisionBufferMM,
	)
	if err != nil {
		return nil, err
	}
	for name, constraint := range fsCollisionConstraints {
		handler.AddStateFSConstraint(name, constraint)
	}

	err = handler.addTopoConstraints(fs, seedMap, startPoses, goalPoses, constraints)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

// addPbConstraints will add all constraints from the passed Constraint struct. This will deal with only the topological
// constraints. It will return a bool indicating whether there are any to add.
func (c *ConstraintChecker) addTopoConstraints(
	fs *referenceframe.FrameSystem,
	startCfg *referenceframe.LinearInputs,
	fromPosesBad, toPoses referenceframe.FrameSystemPoses,
	constraints *Constraints,
) error {
	if len(constraints.LinearConstraint) == 0 &&
		len(constraints.PseudolinearConstraint) == 0 &&
		len(constraints.OrientationConstraint) == 0 {
		return nil
	}

	fromPoses := referenceframe.FrameSystemPoses{}
	for f, b := range fromPosesBad {
		g := toPoses[f]
		if g == nil || b.Parent() == g.Parent() {
			fromPoses[f] = b
			continue
		}
		x, err := fs.Transform(startCfg, referenceframe.NewZeroPoseInFrame(f), g.Parent())
		if err != nil {
			return err
		}
		fromPoses[f] = x.(*referenceframe.PoseInFrame)
	}

	c.AddStateFSConstraint("topo constraint", func(state *StateFS) error {
		for frame, toPIF := range toPoses {
			fromPIF := fromPoses[frame]

			if fromPIF.Parent() != toPIF.Parent() {
				return fmt.Errorf("in topo constraint, from and to are in different frames %s != %s", fromPIF.Parent(), toPIF.Parent())
			}

			currPosePIF, err := state.FS.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), toPIF.Parent())
			if err != nil {
				return err
			}

			from := fromPIF.Pose()
			to := toPIF.Pose()
			currPose := currPosePIF.(*referenceframe.PoseInFrame).Pose()

			for _, lc := range constraints.LinearConstraint {
				err := checkLinearConstraint(frame, lc, from, to, currPose)
				if err != nil {
					return err
				}
			}

			for _, plc := range constraints.PseudolinearConstraint {
				err := checkPseudoLinearConstraint(frame, plc, from, to, currPose)
				if err != nil {
					return err
				}
			}

			for _, oc := range constraints.OrientationConstraint {
				err := checkOrientationConstraint(frame, oc, from, to, currPose)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	return nil
}

func orientationError(prefix string, from, to, curr spatialmath.Orientation, dist, max float64) error {
	return fmt.Errorf("%s %s violated dist: %0.5f > %0.5f from: %v to: %v currPose: %v",
		prefix, orientationConstraintDescription, dist, max,
		from, to, curr)
}

func checkLinearConstraint(frame string, linConstraint LinearConstraint, from, to, currPose spatialmath.Pose) error {
	linTol := linConstraint.LineToleranceMm
	if linTol > 0 {
		dist := spatialmath.DistToLineSegment(from.Point(), to.Point(), currPose.Point())
		if dist > linTol {
			return fmt.Errorf("%s %s violated dist: %0.2f", frame, linearConstraintDescription, dist)
		}
	}
	orientTol := linConstraint.OrientationToleranceDegs
	if orientTol > 0 {
		dist := min(
			OrientDist(from.Orientation(), currPose.Orientation()),
			OrientDist(to.Orientation(), currPose.Orientation()))
		if dist > orientTol {
			return orientationError(frame, from.Orientation(), to.Orientation(), currPose.Orientation(), dist, orientTol)
		}
	}

	return nil
}

func checkPseudoLinearConstraint(frame string, plinConstraint PseudolinearConstraint, from, to, currPose spatialmath.Pose) error {
	linTol := plinConstraint.LineToleranceFactor
	if linTol > 0 {
		linTol *= from.Point().Distance(to.Point())
		dist := spatialmath.DistToLineSegment(from.Point(), to.Point(), currPose.Point())
		if dist > linTol {
			return fmt.Errorf("%s %s violated dist: %0.2f", frame, linearConstraintDescription, dist)
		}
	}

	orientTol := plinConstraint.OrientationToleranceFactor
	if orientTol > 0 {
		orientTol *= OrientDist(from.Orientation(), to.Orientation())
		dist := min(
			OrientDist(from.Orientation(), currPose.Orientation()),
			OrientDist(to.Orientation(), currPose.Orientation()))
		if dist > orientTol {
			return orientationError(frame, from.Orientation(), to.Orientation(), currPose.Orientation(), dist, orientTol)
		}
	}

	return nil
}

func checkOrientationConstraint(frame string, c OrientationConstraint, from, to, currPose spatialmath.Pose) error {
	dist := c.Distance(from.Orientation(), to.Orientation(), currPose.Orientation())
	if dist > c.OrientationToleranceDegs {
		return orientationError(frame, from.Orientation(), to.Orientation(), currPose.Orientation(), dist, c.OrientationToleranceDegs)
	}
	return nil
}

// CheckStateFSConstraints will check a given input against all FS state constraints.
func (c *ConstraintChecker) CheckStateFSConstraints(ctx context.Context, state *StateFS) error {
	_, span := trace.StartSpan(ctx, "CheckStateFSConstraints")
	defer span.End()
	for name, cFunc := range c.stateFSConstraints {
		if err := cFunc(state); err != nil {
			// for better logging, parse out the name of the constraint which is guaranteed to be before the underscore
			return errors.Wrap(err, strings.SplitN(name, "_", 2)[0])
		}
	}
	return nil
}

// InterpolateSegmentFS is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func InterpolateSegmentFS(ci *SegmentFS, resolution float64) ([]*referenceframe.LinearInputs, error) {
	// Find the frame with the most steps by calculating steps for each frame
	maxSteps := defaultMinStepCount
	for frameName, startConfig := range ci.StartConfiguration.Items() {
		if len(startConfig) == 0 {
			// No need to interpolate 0dof frames
			continue
		}
		endConfig := ci.EndConfiguration.Get(frameName)
		if endConfig == nil {
			return nil, fmt.Errorf("frame %s exists in start config but not in end config", frameName)
		}

		// Get frame from FrameSystem
		frame := ci.FS.Frame(frameName)
		if frame == nil {
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
		steps := CalculateStepCount(startPos, endPos, resolution)
		if steps > maxSteps {
			maxSteps = steps
		}
	}

	// Create interpolated configurations for all frames
	var interpolatedConfigurations []*referenceframe.LinearInputs
	for i := 0; i <= maxSteps; i++ {
		interp := float64(i) / float64(maxSteps)
		frameConfigs := referenceframe.NewLinearInputs()

		// Interpolate each frame's configuration
		for frameName, startConfig := range ci.StartConfiguration.Items() {
			endConfig := ci.EndConfiguration.Get(frameName)
			frame := ci.FS.Frame(frameName)

			interpConfig, err := frame.Interpolate(startConfig, endConfig, interp)
			if err != nil {
				return nil, err
			}
			frameConfigs.Put(frameName, interpConfig)
		}

		interpolatedConfigurations = append(interpolatedConfigurations, frameConfigs)
	}

	return interpolatedConfigurations, nil
}

// AddStateFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintChecker) AddStateFSConstraint(name string, cons StateFSConstraint) {
	if c.stateFSConstraints == nil {
		c.stateFSConstraints = map[string]StateFSConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.stateFSConstraints[name] = cons
}

// StateFSConstraints will list all FS state constraints by name.
func (c *ConstraintChecker) StateFSConstraints() []string {
	names := make([]string, 0, len(c.stateFSConstraints))
	for name := range c.stateFSConstraints {
		names = append(names, name)
	}
	return names
}

// CheckStateConstraintsAcrossSegmentFS will interpolate the given input from the StartConfiguration to the EndConfiguration, and ensure
// that all intermediate states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will
// return `true, nil`. If any constraints fail, this will return false, and an SegmentFS representing the valid portion of the segment,
// if any. If no part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintChecker) CheckStateConstraintsAcrossSegmentFS(
	ctx context.Context,
	ci *SegmentFS,
	resolution float64,
) (*SegmentFS, error) {
	ctx, span := trace.StartSpan(ctx, "CheckStateConstraintsAcrossSegmentFS")
	defer span.End()

	interpolatedConfigurations, err := InterpolateSegmentFS(ci, resolution)
	if err != nil {
		return nil, err
	}

	var lastGood *referenceframe.LinearInputs
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &StateFS{FS: ci.FS, Configuration: interpConfig}
		err = c.CheckStateFSConstraints(ctx, interpC)
		if err != nil {
			if i == 0 {
				// fail on start pos
				return nil, err
			}
			return &SegmentFS{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, FS: ci.FS}, err
		}
		lastGood = interpC.Configuration
	}

	return nil, nil
}

// CreateAllCollisionConstraints -.
func CreateAllCollisionConstraints(
	movingRobotGeometries, staticRobotGeometries, worldGeometries []spatialmath.Geometry,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) (map[string]StateFSConstraint, error) {
	constraintFSMap := map[string]StateFSConstraint{}

	if len(worldGeometries) > 0 {
		// create constraint to keep moving geometries from hitting world state obstacles
		obstacleConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			worldGeometries,
			allowedCollisions,
			false,
			collisionBufferMM,
		)
		if err != nil {
			return nil, err
		}
		constraintFSMap[obstacleConstraintDescription] = obstacleConstraintFS
	}

	if len(staticRobotGeometries) > 0 {
		robotConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries,
			staticRobotGeometries,
			allowedCollisions,
			false,
			collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintFSMap[robotCollisionConstraintDescription] = robotConstraintFS
	}

	// create constraint to keep moving geometries from hitting themselves
	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraintFS, err := NewCollisionConstraintFS(
			movingRobotGeometries, nil, allowedCollisions, false, collisionBufferMM)
		if err != nil {
			return nil, err
		}
		constraintFSMap[selfCollisionConstraintDescription] = selfCollisionConstraintFS
	}
	return constraintFSMap, nil
}

func setupZeroCG(
	moving, static []spatialmath.Geometry,
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

// NewCollisionConstraintFS is the most general method to create a collision constraint for a frame system,
// which will be violated if geometries constituting the given frame ever come into collision with obstacle geometries
// outside of the collisions present for the observationInput. Collisions specified as collisionSpecifications will also be ignored.
// If reportDistances is false, this check will be done as fast as possible, if true maximum information will be available for debugging.
func NewCollisionConstraintFS(
	moving, static []spatialmath.Geometry,
	collisionSpecifications []*Collision,
	reportDistances bool,
	collisionBufferMM float64,
) (StateFSConstraint, error) {
	zeroCG, err := setupZeroCG(moving, static, collisionSpecifications, true, collisionBufferMM)
	if err != nil {
		return nil, err
	}

	movingMap := map[string]spatialmath.Geometry{}
	for _, geom := range moving {
		movingMap[geom.Label()] = geom
	}

	// create constraint from reference collision graph
	constraint := func(state *StateFS) error {
		// Use FrameSystemGeometries to get all geometries in the frame system
		internalGeometries, err := state.Geometries()
		if err != nil {
			return err
		}

		// We only want to compare *moving* geometries, so we filter what we get from the framesystem against what we were passed.
		var internalGeoms []spatialmath.Geometry
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

func collisionCheckFinish(internalGeoms, static []spatialmath.Geometry, zeroCG *collisionGraph,
	reportDistances bool, collisionBufferMM float64,
) error {
	cg, err := newCollisionGraph(internalGeoms, static, zeroCG, reportDistances, collisionBufferMM)
	if err != nil {
		return err
	}
	cs := cg.collisions(collisionBufferMM)
	if len(cs) != 0 {
		// we could choose to amalgamate all the collisions into one error but its probably saner not to and choose just the first
		return fmt.Errorf("violation between %s and %s geometries (total collisions: %d)", cs[0].name1, cs[0].name2, len(cs))
	}
	return nil
}
