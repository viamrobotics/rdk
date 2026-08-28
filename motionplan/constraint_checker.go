package motionplan

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// short descriptions of constraints used in error messages.
const (
	linearConstraintDescription      = "linear constraint"
	orientationConstraintDescription = "orientation constraint"
	planarConstraintDescription      = "planar constraint"

	// collision constraint descriptions used in error messages.
	boundingRegionConstraintDescription = "bounding region constraint"
	ObstacleConstraintDescription       = "obstacle constraint"
	selfCollisionConstraintDescription  = "self-collision constraint"
	RobotCollisionConstraintDescription = "robot constraint" // collision between a moving robot component and one that is stationary

	defaultCollisionBufferMM = 1e-8
	defaultMinStepCount      = 2
)

// StateFSConstraint tests whether a given robot configuration is valid
// If the returned error is nil, the constraint is satisfied and the state is valid.
type StateFSConstraint func(*StateFS) error

// CollisionConstraintFunc tests if there is a collision
// first return is closest target
type CollisionConstraintFunc func(*StateFS) (float64, error)

// CollisionConstraints holds the three types of collision constraints that may be active during planning.
type CollisionConstraints struct {
	Obstacle      CollisionConstraintFunc // moving geometries vs world obstacles
	RobotToRobot  CollisionConstraintFunc // moving robot component vs stationary robot component
	SelfCollision CollisionConstraintFunc // moving geometries vs themselves
}

// ConstraintChecker is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type ConstraintChecker struct {
	collisionConstraints CollisionConstraints
	topoConstraint       StateFSConstraint

	logger logging.Logger
}

// NewEmptyConstraintChecker - creates a ConstraintChecker with nothing.
func NewEmptyConstraintChecker(logger logging.Logger) *ConstraintChecker {
	return &ConstraintChecker{logger: logger}
}

// NewConstraintChecker - creates a ConstraintChecker with all the params.
// When cache is non-nil, the resulting constraint closures use it for
// temporal-coherence short-circuits (pair hints + per-triangle witnesses).
// Typically supplied by the planner from planContext.cache; nil disables caching.
func NewConstraintChecker(
	collisionBufferMM float64,
	constraints *Constraints,
	startPoses, goalPoses referenceframe.FrameSystemPoses,
	fs *referenceframe.FrameSystem,
	movingRobotGeometries, staticRobotGeometries []spatialmath.Geometry,
	movingFrameNames map[string]bool,
	seedMap *referenceframe.LinearInputs,
	obstaclesInWorldFrame *referenceframe.GeometriesInFrame,
	logger logging.Logger,
	cache *CollisionCache,
) (*ConstraintChecker, error) {
	if constraints == nil {
		// Constraints may be nil, but if a motion profile is set in planningOpts
		// we need it to be a valid pointer to an empty struct.
		constraints = &Constraints{}
	}
	handler := NewEmptyConstraintChecker(logger)

	frameSystemGeometries, err := referenceframe.FrameSystemGeometriesLinearInputs(fs, seedMap)
	if err != nil {
		return nil, err
	}

	var worldGeometries []spatialmath.Geometry
	if obstaclesInWorldFrame != nil {
		worldGeometries = obstaclesInWorldFrame.Geometries()
	}

	obstacleNames := make(map[string]bool)
	for _, geometry := range worldGeometries {
		obstacleNames[geometry.Label()] = true
	}

	frameNames := map[string]bool{}
	for _, fName := range fs.FrameNames() {
		frameNames[fName] = true
	}

	allowedCollisions, err := collisionSpecifications(
		constraints.CollisionSpecification,
		frameSystemGeometries,
		frameNames,
		obstacleNames,
	)
	if err != nil {
		return nil, err
	}

	// add collision constraints
	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs,
		movingRobotGeometries,
		movingFrameNames,
		staticRobotGeometries,
		worldGeometries,
		allowedCollisions,
		collisionBufferMM,
		cache,
		logger,
	)
	if err != nil {
		return nil, err
	}

	err = handler.addTopoConstraints(fs, seedMap, startPoses, goalPoses, constraints)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

// SetCollisionConstraints set the collision constraints explicitly
func (c *ConstraintChecker) SetCollisionConstraints(cs CollisionConstraints) {
	c.collisionConstraints = cs
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

	// Precompute per-frame orientation-constraint evaluators: the endpoint
	// orientation-vector conversions are the expensive part of the check and
	// the endpoints are fixed for the plan's lifetime.
	orientationEvals := map[string][]*OrientationConstraintEval{}
	for frame, toPIF := range toPoses {
		fromPIF := fromPoses[frame]
		if fromPIF.Parent() != toPIF.Parent() {
			return fmt.Errorf("in topo constraint, from and to are in different frames %s != %s", fromPIF.Parent(), toPIF.Parent())
		}
		for _, oc := range constraints.OrientationConstraint {
			orientationEvals[frame] = append(orientationEvals[frame],
				NewOrientationConstraintEval(oc, fromPIF.Pose().Orientation(), toPIF.Pose().Orientation()))
		}
	}

	c.topoConstraint = func(state *StateFS) error {
		for frame, toPIF := range toPoses {
			fromPIF := fromPoses[frame]

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

			for _, eval := range orientationEvals[frame] {
				err := checkOrientationConstraintEval(frame, eval, currPose)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	return nil
}

func orientationError(prefix string, from, to, curr spatialmath.Orientation, dist, max float64) error { //nolint: revive
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

func checkOrientationConstraintEval(frame string, e *OrientationConstraintEval, currPose spatialmath.Pose) error {
	dist := e.Distance(currPose.Orientation())
	if dist > e.oc.OrientationToleranceDegs {
		return orientationError(frame, e.from, e.to, currPose.Orientation(), dist, e.oc.OrientationToleranceDegs)
	}
	return nil
}

// CheckStateFSConstraints will check a given input against all FS state constraints.
// first is closest obstacle, negative if in collision
// Deliberately no tracing span here: this runs hundreds of thousands of times
// per plan, and per-call span creation plus exporter wakeups measurably
// dominated planning time once the checks themselves became cheap.
func (c *ConstraintChecker) CheckStateFSConstraints(ctx context.Context, state *StateFS) (float64, error) {
	// The cover evaluation is shared by the collision constraints below and
	// recycled once this state check completes.
	defer releaseCoverSet(state)
	// Topological constraints (orientation/linear) cost a single FK pass —
	// orders of magnitude cheaper than the collision sweeps below — so check
	// them first: constrained planners generate many candidate states that fail
	// only the topological check, and those shouldn't pay for collision work.
	if c.topoConstraint != nil {
		if err := c.topoConstraint(state); err != nil {
			return math.Inf(1), err
		}
	}

	closest := math.Inf(1)

	for _, pair := range []struct {
		name string
		fn   CollisionConstraintFunc
	}{
		{ObstacleConstraintDescription, c.collisionConstraints.Obstacle},
		{RobotCollisionConstraintDescription, c.collisionConstraints.RobotToRobot},
		{selfCollisionConstraintDescription, c.collisionConstraints.SelfCollision},
	} {
		if pair.fn == nil {
			continue
		}
		d, err := pair.fn(state)
		closest = min(closest, d)
		if err != nil {
			return -1, errors.Wrap(err, pair.name)
		}
	}

	return closest, nil
}

// CheckStateFSTopoOnly evaluates only the topological (orientation/linear)
// constraints for a state — the cheap, single-FK subset of
// CheckStateFSConstraints. For callers that skip collision checks under a
// clearance bound but must still verify topological validity per state.
func (c *ConstraintChecker) CheckStateFSTopoOnly(state *StateFS) error {
	if c.topoConstraint == nil {
		return nil
	}
	return c.topoConstraint(state)
}

// InterpolateSegmentFS is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func InterpolateSegmentFS(ci *SegmentFS, resolution float64) ([]*referenceframe.LinearInputs, error) {
	// Find the frame with the most steps by calculating steps for each frame
	maxSteps, err := segmentStepCount(ci, resolution)
	if err != nil {
		return nil, err
	}

	// Create interpolated configurations for all frames
	var interpolatedConfigurations []*referenceframe.LinearInputs
	for i := 0; i <= maxSteps; i++ {
		interp := float64(i) / float64(maxSteps)
		frameConfigs := referenceframe.NewLinearInputs()

		// Interpolate each frame's configuration
		for frameName, startConfig := range ci.StartConfiguration.Items() {
			// 0-DoF frames have nothing to interpolate.
			if len(startConfig) == 0 {
				continue
			}
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

// CheckStateConstraintsAcrossSegmentFS will interpolate the given input from the StartConfiguration to the EndConfiguration, and ensure
// that all intermediate states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will
// return `true, nil`. If any constraints fail, this will return false, and an SegmentFS representing the valid portion of the segment,
// if any. If no part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintChecker) CheckStateConstraintsAcrossSegmentFS(
	ctx context.Context,
	ci *SegmentFS,
	resolution float64,
	checkFinal bool,
) (*SegmentFS, error) {
	interpolatedConfigurations, err := InterpolateSegmentFS(ci, resolution)
	if err != nil {
		return nil, err
	}

	var lastGood *referenceframe.LinearInputs

	end := len(interpolatedConfigurations)
	if !checkFinal {
		end--
	}
	for i := 0; i < end; i++ {
		interpConfig := interpolatedConfigurations[i]
		interpC := &StateFS{FS: ci.FS, Configuration: interpConfig}
		closestObstacle, err := c.CheckStateFSConstraints(ctx, interpC)
		if err != nil {
			if i == 0 {
				// fail on start pos
				return nil, err
			}
			return &SegmentFS{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, FS: ci.FS}, err
		}
		lastGood = interpC.Configuration

		// The clearance at this state proves the next canSkip states cannot
		// collide (the robot can't cross a gap faster than it moves), so the
		// expensive collision constraints may be skipped for them. Topological
		// constraints (orientation/linear) carry no such guarantee but are
		// cheap - a single FK pass per state - so evaluate just those on the
		// skipped states instead of disabling skipping altogether.
		canSkip := int(min(100, math.Floor(closestObstacle/resolution)))
		if canSkip > 0 && c.topoConstraint != nil {
			for j := i + 1; j <= i+canSkip && j < end; j++ {
				topoC := &StateFS{FS: ci.FS, Configuration: interpolatedConfigurations[j]}
				if err := c.topoConstraint(topoC); err != nil {
					return &SegmentFS{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, FS: ci.FS}, err
				}
				// Collision-free by the clearance bound and topo-checked: valid.
				lastGood = topoC.Configuration
			}
		}
		if canSkip > 0 {
			i += canSkip
		}
	}

	return nil, nil
}

// CreateAllCollisionConstraints builds the three collision constraint
// functions (obstacle / robot-vs-robot / self-collision). When cache is non-nil,
// each constraint registers its pair labels for witness-slot caching and uses
// its dedicated pair-hint slot.
func CreateAllCollisionConstraints(
	fs *referenceframe.FrameSystem,
	movingRobotGeometries []spatialmath.Geometry,
	movingFrameNames map[string]bool,
	staticRobotGeometries, worldGeometries []spatialmath.Geometry,
	allowedCollisions []Collision,
	collisionBufferMM float64,
	cache *CollisionCache,
	logger logging.Logger,
) (CollisionConstraints, error) {
	var constraints CollisionConstraints

	// Each constraint type gets its own pair-hint slot from the cache so the
	// three hints don't trample each other across constraint invocations.
	var obstacleHint, robotHint, selfHint *atomic.Pointer[[2]string]
	if cache != nil {
		obstacleHint = &cache.obstaclePairHint
		robotHint = &cache.robotPairHint
		selfHint = &cache.selfPairHint
	}

	// Precompute local-frame sphere covers for the moving frames once; the
	// three constraint closures share the table (and, per state, the world
	// covers computed from it) so most states never materialize geometry.
	coverTable := buildFrameCoverTable(fs, movingFrameNames)

	if len(worldGeometries) > 0 {
		obstacleConstraintFS, err := NewCollisionConstraintFS(
			fs,
			movingRobotGeometries,
			movingFrameNames,
			worldGeometries,
			allowedCollisions,
			collisionBufferMM,
			false,
			obstacleHint,
			cache,
			coverTable,
			logger,
		)
		if err != nil {
			return CollisionConstraints{}, err
		}
		constraints.Obstacle = obstacleConstraintFS
	}

	if len(staticRobotGeometries) > 0 {
		robotConstraintFS, err := NewCollisionConstraintFS(
			fs,
			movingRobotGeometries,
			movingFrameNames,
			staticRobotGeometries,
			allowedCollisions,
			collisionBufferMM,
			false,
			robotHint,
			cache,
			coverTable,
			logger,
		)
		if err != nil {
			return CollisionConstraints{}, err
		}
		constraints.RobotToRobot = robotConstraintFS
	}

	if len(movingRobotGeometries) > 1 {
		selfCollisionConstraintFS, err := NewCollisionConstraintFS(
			fs,
			movingRobotGeometries,
			movingFrameNames,
			movingRobotGeometries,
			allowedCollisions,
			collisionBufferMM,
			true,
			selfHint,
			cache,
			coverTable,
			logger,
		)
		if err != nil {
			return CollisionConstraints{}, err
		}
		constraints.SelfCollision = selfCollisionConstraintFS
	}
	return constraints, nil
}

// NewCollisionConstraintFS is the most general method to create a collision constraint for a frame system,
// which will be violated if geometries constituting the given frame ever come into collision with obstacle geometries
// outside of the collisions present for the observationInput. Collisions specified as collisionSpecifications will also be ignored.
//
// When pairHint is non-nil, the closure uses it to remember the most-recently-
// violated (geomA, geomB) pair and try that pair first on the next call.
func NewCollisionConstraintFS(
	fs *referenceframe.FrameSystem,
	moving []spatialmath.Geometry,
	movingFrameNames map[string]bool,
	static []spatialmath.Geometry,
	collisionSpecifications []Collision,
	collisionBufferMM float64,
	isSelfCollision bool,
	pairHint *atomic.Pointer[[2]string],
	cache *CollisionCache,
	coverTable *frameCoverTable,
	logger logging.Logger,
) (CollisionConstraintFunc, error) {
	ignoreCollisions, err := computeInitialCollisionsToIgnore(fs, moving, static,
		collisionSpecifications, collisionBufferMM, logger)
	if err != nil {
		return nil, err
	}

	allowed := makeAllowedCollisionsLookup(ignoreCollisions)

	// Distance-field fast path for the moving-vs-static product: a voxel SDF
	// over the static set answers "this moving geometry is at least D away
	// from everything static" from a handful of array lookups against the
	// geometry's conservative sphere cover. Only clear verdicts are used -
	// anything near, uncovered, or colliding falls through to the exact
	// checker, so allowed-collision pairs and near-contact distances keep
	// their exact semantics. The field is built once per static scene and
	// shared across all segment contexts through the plan cache.
	var sdf *spatialmath.VoxelSDF
	if !isSelfCollision {
		sdf = cache.SDFFor(static)
	}
	sdfClearThreshold := math.Max(2.0, 2*collisionBufferMM)

	// The static side of every pair check is the same geometry set at the
	// same poses for the life of this checker; prebuilding its names, allow
	// flags, and bounding spheres removes that work from every state check.
	staticPre := prenameGeoms(static, allowed)

	// finish shares the tail of every path: convert a found collision into
	// the constraint-violated error.
	finish := func(collisions []Collision, minDist float64, err error) (float64, error) {
		if err != nil {
			return minDist, err
		}
		if len(collisions) != 0 {
			return minDist, fmt.Errorf(
				"violation between %s and %s geometries",
				collisions[0].name1,
				collisions[0].name2,
			)
		}
		return minDist, nil
	}

	// checkStaticViaCovers evaluates the moving-vs-static constraint from the
	// state's sphere covers and the distance field, materializing geometry
	// only for the frames the sphere phase cannot clear.
	checkStaticViaCovers := func(state *StateFS, cs *stateCoverSet) (float64, error) {
		sdfMin := math.Inf(1)
		needFrames := map[string]bool{}
		for f := range coverTable.materialize {
			needFrames[f] = true
		}
		for i := range cs.geoms {
			cg := &cs.geoms[i]
			lb := math.Inf(1)
			for _, sb := range cg.world {
				if v := sdf.PointClearance(sb.Center) - sb.R; v < lb {
					lb = v
				}
			}
			if lb > sdfClearThreshold {
				sdfMin = math.Min(sdfMin, lb)
			} else {
				needFrames[cg.frameName] = true
			}
		}
		if len(needFrames) == 0 {
			return sdfMin, nil
		}
		geoms, err := materializedSubset(state, fs, needFrames)
		if err != nil {
			return 0, err
		}
		collisions, minDist, err := checkCollisionsHinted(
			geoms, static, staticPre, allowed, collisionBufferMM, false, pairHint, logger)
		return finish(collisions, math.Min(minDist, sdfMin), err)
	}

	// checkSelfViaCovers evaluates self-collision pairwise from the covers'
	// enclosing spheres; only frames belonging to a near pair (or without
	// covers) are materialized and checked exactly.
	checkSelfViaCovers := func(state *StateFS, cs *stateCoverSet) (float64, error) {
		selfMin := math.Inf(1)
		near := map[string]bool{}
		for f := range coverTable.materialize {
			near[f] = true
		}
		// Uncovered frames must pair against everything; materialize them
		// first so their bounding spheres can prune covered partners.
		var uncoveredBounds []spatialmath.SphereBound
		if len(near) > 0 {
			geoms, err := materializedSubset(state, fs, near)
			if err != nil {
				return 0, err
			}
			for _, g := range geoms {
				if c, r, ok := spatialmath.BoundingSphere(g); ok {
					uncoveredBounds = append(uncoveredBounds, spatialmath.SphereBound{Center: c, R: r})
				} else {
					// No cheap bound: everything is potentially near it.
					for i := range cs.geoms {
						near[cs.geoms[i].frameName] = true
					}
					uncoveredBounds = nil
					break
				}
			}
		}
		for i := range cs.geoms {
			a := &cs.geoms[i]
			for j := i + 1; j < len(cs.geoms); j++ {
				b := &cs.geoms[j]
				gap := a.bound.Center.Sub(b.bound.Center).Norm() - a.bound.R - b.bound.R
				if gap > sdfClearThreshold {
					selfMin = math.Min(selfMin, gap)
					continue
				}
				near[a.frameName] = true
				near[b.frameName] = true
			}
			for _, ub := range uncoveredBounds {
				if a.bound.Center.Sub(ub.Center).Norm()-a.bound.R-ub.R <= sdfClearThreshold {
					near[a.frameName] = true
				}
			}
		}
		if len(near) == 0 {
			return selfMin, nil
		}
		geoms, err := materializedSubset(state, fs, near)
		if err != nil {
			return 0, err
		}
		collisions, minDist, err := checkCollisionsHinted(
			geoms, geoms, nil, allowed, collisionBufferMM, false, pairHint, logger)
		return finish(collisions, math.Min(minDist, selfMin), err)
	}

	// create constraint from reference collision graph
	constraint := func(state *StateFS) (float64, error) {
		if coverTable != nil {
			if cs := coverSetFor(state, fs, coverTable); !cs.failed {
				if isSelfCollision {
					return checkSelfViaCovers(state, cs)
				}
				if sdf != nil {
					return checkStaticViaCovers(state, cs)
				}
			}
		}

		// state.movingGeometries is shared across the three collision constraints
		// (obstacle / robot-vs-robot / self-collision); whichever runs first for
		// this state populates it, the others hit the cache. Safe because all
		// three closures captured the same movingFrameNames.
		if state.movingGeometries == nil {
			g, err := referenceframe.FrameSystemGeometriesForFrames(state.FS, state.Configuration, movingFrameNames)
			if err != nil {
				return 0, err
			}
			state.movingGeometries = g
		}

		var internalGeoms []spatialmath.Geometry
		for _, geosInFrame := range state.movingGeometries {
			internalGeoms = append(internalGeoms, geosInFrame.Geometries()...)
		}

		sdfMin := math.Inf(1)
		if sdf != nil {
			// Keep only the geometries the SDF cannot clear for the exact pass.
			needExact := internalGeoms[:0:0]
			for _, g := range internalGeoms {
				cover := spatialmath.SphereCover(g)
				if cover == nil {
					needExact = append(needExact, g)
					continue
				}
				lb := math.Inf(1)
				for _, sb := range cover {
					if v := sdf.PointClearance(sb.Center) - sb.R; v < lb {
						lb = v
					}
				}
				if lb > sdfClearThreshold {
					sdfMin = math.Min(sdfMin, lb)
				} else {
					needExact = append(needExact, g)
				}
			}
			if len(needExact) == 0 {
				return sdfMin, nil
			}
			internalGeoms = needExact
		}

		// For self-collision, compare moving geometries against themselves
		staticToCheck := static
		if isSelfCollision {
			staticToCheck = internalGeoms
		}

		staticPreToUse := staticPre
		if isSelfCollision {
			staticPreToUse = nil
		}
		collisions, minDist, err := checkCollisionsHinted(
			internalGeoms, staticToCheck, staticPreToUse, allowed, collisionBufferMM, false, pairHint, logger)
		minDist = math.Min(minDist, sdfMin)
		if err != nil {
			return minDist, err
		}
		if len(collisions) != 0 {
			return minDist, fmt.Errorf(
				"violation between %s and %s geometries",
				collisions[0].name1,
				collisions[0].name2,
			)
		}
		return minDist, nil
	}
	return constraint, nil
}

func computeInitialCollisionsToIgnore(
	fs *referenceframe.FrameSystem,
	group1, group2 []spatialmath.Geometry,
	collisionSpecifications []Collision,
	collisionBufferMM float64,
	logger logging.Logger,
) ([]Collision, error) {
	// Geometries in collision at move start should thereafter be ignored
	initialCollisions, _, err := checkCollisionsHinted(
		group1, group2, nil, makeAllowedCollisionsLookup(collisionSpecifications), collisionBufferMM, true, nil, logger)
	if err != nil {
		return nil, err
	}

	// Add collision specifications
	initialCollisions = append(initialCollisions, collisionSpecifications...)

	// Add coparented static frames that could never be brought into collision
	initialCollisions = append(initialCollisions, findCoparentedStaticFrames(fs, group1, group2)...)

	return initialCollisions, nil
}

func findCoparentedStaticFrames(fs *referenceframe.FrameSystem, group1, group2 []spatialmath.Geometry) []Collision {
	skipList := []Collision{}

	// Build a reverse lookup for geometries whose label is namespaced under a
	// frame (e.g. "follower-gripper:gripper_body_0" on the "follower-gripper"
	// frame). Only frames with zero DoF qualify — for kinematic models (e.g.
	// SimpleModel) all link geometries appear under one frame name in the
	// frame system, but they're separated by internal joints and do NOT share
	// rigid motion.
	geomLabelToStaticFrame := map[string]referenceframe.Frame{}
	for _, name := range fs.FrameNames() {
		f := fs.Frame(name)
		if f == nil || len(f.DoF()) != 0 {
			continue
		}
		gif, err := f.Geometries(nil)
		if err != nil || gif == nil {
			continue
		}
		for _, g := range gif.Geometries() {
			geomLabelToStaticFrame[g.Label()] = f
		}
	}
	resolveFrame := func(geomLabel string) referenceframe.Frame {
		if f := fs.Frame(geomLabel); f != nil {
			return f
		}
		return geomLabelToStaticFrame[geomLabel]
	}

	for _, g1 := range group1 {
		g1Name := g1.Label()
		for _, g2 := range group2 {
			g2Name := g2.Label()
			if g1Name == g2Name {
				continue
			}

			x := resolveFrame(g1Name)
			y := resolveFrame(g2Name)

			if x == nil || y == nil {
				// Geometry not in frame system (e.g. internal to a component), must check for collision
				continue
			}

			if fs.SharesRigidMotion(x, y) {
				skipList = append(skipList, Collision{name1: g1Name, name2: g2Name})
			}
		}
	}
	return skipList
}

// Computes the quantity of intermediate constraint check steps that should be performed across a segment
func segmentStepCount(ci *SegmentFS, resolution float64) (int, error) {
	// Find the frame with the most steps by calculating steps for each frame
	maxSteps := defaultMinStepCount
	for frameName, startConfig := range ci.StartConfiguration.Items() {
		if len(startConfig) == 0 {
			// No need to interpolate 0dof frames
			continue
		}
		endConfig := ci.EndConfiguration.Get(frameName)
		if endConfig == nil {
			return -1, fmt.Errorf("frame %s exists in start config but not in end config", frameName)
		}

		// Get frame from FrameSystem
		frame := ci.FS.Frame(frameName)
		if frame == nil {
			return -1, fmt.Errorf("frame %s exists in start config but not in framesystem", frameName)
		}

		// Calculate positions for this frame's start and end configs
		startPos, err := frame.Transform(startConfig)
		if err != nil {
			return -1, err
		}
		endPos, err := frame.Transform(endConfig)
		if err != nil {
			return -1, err
		}

		// Compute joint step size from the largest limit range, divided by 1000
		jointStepSize := jointStepSizeFromLimits(frame.DoF())

		maxSteps = max(maxSteps, CalculateStepCount(startPos, endPos, resolution))
		maxSteps = max(maxSteps, calculateJointStepCount(startConfig, endConfig, jointStepSize))
	}
	return maxSteps, nil
}

// jointStepSizeFromLimits computes the joint step size as 1/1000 of the largest limit range.
func jointStepSizeFromLimits(limits []referenceframe.Limit) float64 {
	var maxRange float64

	for _, limit := range limits {
		_, _, r := limit.GoodLimits()
		maxRange = max(maxRange, r)
	}
	return maxRange / 1000
}
