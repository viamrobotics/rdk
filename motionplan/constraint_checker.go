package motionplan

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var defaultMinStepCount = 2

// ConstraintChecker is a convenient wrapper for constraint handling which is likely to be common among most motion
// planners. Including a constraint handler as an anonymous struct member allows reuse.
type ConstraintChecker struct {
	segmentConstraints   map[string]SegmentConstraint
	segmentFSConstraints map[string]SegmentFSConstraint
	stateConstraints     map[string]StateConstraint
	stateFSConstraints   map[string]StateFSConstraint
	pathMetric           StateFSMetric // Distance function which converges on the valid manifold of intermediate path states
	boundingRegions      []spatialmath.Geometry
}

// NewEmptyConstraintChecker - creates a ConstraintChecker with nothing.
func NewEmptyConstraintChecker() *ConstraintChecker {
	handler := ConstraintChecker{}
	handler.pathMetric = NewZeroFSMetric()
	return &handler
}

// NewConstraintCheckerWithPathMetric - creates a ConstraintChecker with a specific metric.
func NewConstraintCheckerWithPathMetric(m StateFSMetric) *ConstraintChecker {
	handler := ConstraintChecker{}
	handler.pathMetric = m
	return &handler
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
	boundingRegions []spatialmath.Geometry,
	useTPspace bool,
) (*ConstraintChecker, error) {
	if constraints == nil {
		// Constraints may be nil, but if a motion profile is set in planningOpts
		// we need it to be a valid pointer to an empty struct.
		constraints = &Constraints{}
	}
	handler := NewEmptyConstraintChecker()
	handler.boundingRegions = boundingRegions

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
		constraints.GetCollisionSpecification(),
		frameSystemGeometries,
		frameNames,
		worldState.ObstacleNames(),
	)
	if err != nil {
		return nil, err
	}

	if useTPspace {
		var zeroCG *collisionGraph
		for _, geom := range worldGeometries {
			if octree, ok := geom.(*pointcloud.BasicOctree); ok {
				if zeroCG == nil {
					zeroCG, err = setupZeroCG(movingRobotGeometries, worldGeometries, allowedCollisions, false, collisionBufferMM)
					if err != nil {
						return nil, err
					}
				}
				// Check if a moving geometry is in collision with a pointcloud. If so, error.
				for _, collision := range zeroCG.collisions(collisionBufferMM) {
					if collision.name1 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name2)
					} else if collision.name2 == octree.Label() {
						return nil, fmt.Errorf("starting collision between SLAM map and %s, cannot move", collision.name1)
					}
				}
			}
		}
	}

	// add collision constraints
	fsCollisionConstraints, stateCollisionConstraints, err := CreateAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries,
		boundingRegions,
		allowedCollisions,
		collisionBufferMM,
	)
	if err != nil {
		return nil, err
	}
	// For TPspace
	for name, constraint := range stateCollisionConstraints {
		handler.AddStateConstraint(name, constraint)
	}
	for name, constraint := range fsCollisionConstraints {
		handler.AddStateFSConstraint(name, constraint)
	}

	_, err = handler.addTopoConstraints(fs, seedMap, startPoses, goalPoses, constraints)
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
	from, to referenceframe.FrameSystemPoses,
	constraints *Constraints,
) (bool, error) {
	topoConstraints := false
	for _, linearConstraint := range constraints.GetLinearConstraint() {
		topoConstraints = true
		// TODO RSDK-9224: Our proto for constraints does not allow the specification of which frames should be constrainted relative to
		// which other frames. If there is only one goal specified, then we assume that the constraint is between the moving and goal frame.
		err := c.addLinearConstraints(fs, startCfg, from, to, linearConstraint)
		if err != nil {
			return false, err
		}
	}
	for _, pseudolinearConstraint := range constraints.GetPseudolinearConstraint() {
		// pseudolinear constraints
		err := c.addPseudolinearConstraints(fs, startCfg, from, to, pseudolinearConstraint)
		if err != nil {
			return false, err
		}
	}
	for _, orientationConstraint := range constraints.GetOrientationConstraint() {
		topoConstraints = true
		// TODO RSDK-9224: Our proto for constraints does not allow the specification of which frames should be constrainted relative to
		// which other frames. If there is only one goal specified, then we assume that the constraint is between the moving and goal frame.
		err := c.addOrientationConstraints(fs, startCfg, from, to, orientationConstraint)
		if err != nil {
			return false, err
		}
	}
	return topoConstraints, nil
}

func (c *ConstraintChecker) addLinearConstraints(
	fs *referenceframe.FrameSystem,
	startCfg *referenceframe.LinearInputs,
	from, to referenceframe.FrameSystemPoses,
	linConstraint LinearConstraint,
) error {
	// Linear constraints
	linTol := linConstraint.LineToleranceMm
	if linTol == 0 {
		// Default
		linTol = defaultLinearDeviation
	}
	orientTol := linConstraint.OrientationToleranceDegs
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist, err := createAbsoluteLinearInterpolatingConstraintFS(fs, startCfg, from, to, linTol, orientTol)
	if err != nil {
		return err
	}
	c.AddStateFSConstraint(defaultConstraintName, constraint)

	c.pathMetric = CombineFSMetrics(c.pathMetric, pathDist)
	return nil
}

func (c *ConstraintChecker) addPseudolinearConstraints(
	fs *referenceframe.FrameSystem,
	startCfg *referenceframe.LinearInputs,
	from, to referenceframe.FrameSystemPoses,
	plinConstraint PseudolinearConstraint,
) error {
	// Linear constraints
	linTol := plinConstraint.LineToleranceFactor
	if linTol == 0 {
		// Default
		linTol = defaultPseudolinearTolerance
	}
	orientTol := plinConstraint.OrientationToleranceFactor
	if orientTol == 0 {
		orientTol = defaultPseudolinearTolerance
	}
	constraint, pathDist, err := createProportionalLinearInterpolatingConstraintFS(fs, startCfg, from, to, linTol, orientTol)
	if err != nil {
		return err
	}
	c.AddStateFSConstraint(defaultConstraintName, constraint)

	c.pathMetric = CombineFSMetrics(c.pathMetric, pathDist)
	return nil
}

func (c *ConstraintChecker) addOrientationConstraints(
	fs *referenceframe.FrameSystem,
	startCfg *referenceframe.LinearInputs,
	from, to referenceframe.FrameSystemPoses,
	orientConstraint OrientationConstraint,
) error {
	orientTol := orientConstraint.OrientationToleranceDegs
	if orientTol == 0 {
		orientTol = defaultOrientationDeviation
	}
	constraint, pathDist, err := createSlerpOrientationConstraintFS(fs, startCfg, from, to, orientTol)
	if err != nil {
		return err
	}
	c.AddStateFSConstraint(defaultConstraintName, constraint)
	c.pathMetric = CombineFSMetrics(c.pathMetric, pathDist)
	return nil
}

// CheckStateConstraints will check a given input against all state constraints.
func (c *ConstraintChecker) CheckStateConstraints(state *State) error {
	for name, cFunc := range c.stateConstraints {
		if err := cFunc(state); err != nil {
			// for better logging, parse out the name of the constraint which is guaranteed to be before the underscore
			return errors.Wrap(err, strings.SplitN(name, "_", 2)[0])
		}
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

// CheckSegmentConstraints will check a given input against all segment constraints.
func (c *ConstraintChecker) CheckSegmentConstraints(segment *Segment) error {
	for name, cFunc := range c.segmentConstraints {
		if err := cFunc(segment); err != nil {
			// for better logging, parse out the name of the constraint which is guaranteed to be before the underscore
			return errors.Wrap(err, strings.SplitN(name, "_", 2)[0])
		}
	}
	return nil
}

// CheckSegmentFSConstraints will check a given input against all FS segment constraints.
func (c *ConstraintChecker) CheckSegmentFSConstraints(segment *SegmentFS) error {
	for name, cFunc := range c.segmentFSConstraints {
		if err := cFunc(segment); err != nil {
			// for better logging, parse out the name of the constraint which is guaranteed to be before the underscore
			return errors.Wrap(err, strings.SplitN(name, "_", 2)[0])
		}
	}
	return nil
}

// CheckStateConstraintsAcrossSegment will interpolate the given input from the StartInput to the EndInput, and ensure that all intermediate
// states as well as both endpoints satisfy all state constraints. If all constraints are satisfied, then this will return `true, nil`.
// If any constraints fail, this will return false, and an Segment representing the valid portion of the segment, if any. If no
// part of the segment is valid, then `false, nil` is returned.
func (c *ConstraintChecker) CheckStateConstraintsAcrossSegment(ci *Segment, resolution float64) (bool, *Segment) {
	interpolatedConfigurations, err := InterpolateSegment(ci, resolution)
	if err != nil {
		return false, nil
	}
	var lastGood []referenceframe.Input
	for i, interpConfig := range interpolatedConfigurations {
		interpC := &State{Frame: ci.Frame, Configuration: interpConfig}
		if interpC.ResolveStateAndUpdatePositions() != nil {
			return false, nil
		}
		if c.CheckStateConstraints(interpC) != nil {
			if i == 0 {
				// fail on start pos
				return false, nil
			}
			return false, &Segment{StartConfiguration: ci.StartConfiguration, EndConfiguration: lastGood, Frame: ci.Frame}
		}
		lastGood = interpC.Configuration
	}

	return true, nil
}

// InterpolateSegment is a helper function which produces a list of intermediate inputs, between the start and end
// configuration of a segment at a given resolution value.
func InterpolateSegment(ci *Segment, resolution float64) ([][]referenceframe.Input, error) {
	// ensure we have cartesian positions
	if err := resolveSegmentsToPositions(ci); err != nil {
		return nil, err
	}

	steps := CalculateStepCount(ci.StartPosition, ci.EndPosition, resolution)
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

// CheckSegmentAndStateValidity will check an segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintChecker) CheckSegmentAndStateValidity(segment *Segment, resolution float64) (bool, *Segment) {
	valid, subSegment := c.CheckStateConstraintsAcrossSegment(segment, resolution)
	if !valid {
		if subSegment != nil {
			if c.CheckSegmentConstraints(subSegment) == nil {
				return false, subSegment
			}
		}
		return false, nil
	}
	// all states are valid
	return c.CheckSegmentConstraints(segment) == nil, nil
}

// AddStateConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintChecker) AddStateConstraint(name string, cons StateConstraint) {
	if c.stateConstraints == nil {
		c.stateConstraints = map[string]StateConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.stateConstraints[name] = cons
}

// StateConstraints will list all state constraints by name.
func (c *ConstraintChecker) StateConstraints() []string {
	names := make([]string, 0, len(c.stateConstraints))
	for name := range c.stateConstraints {
		names = append(names, name)
	}
	return names
}

// AddSegmentConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintChecker) AddSegmentConstraint(name string, cons SegmentConstraint) {
	if c.segmentConstraints == nil {
		c.segmentConstraints = map[string]SegmentConstraint{}
	}
	// Add function address to name to prevent collisions
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.segmentConstraints[name] = cons
}

// SegmentConstraints will list all segment constraints by name.
func (c *ConstraintChecker) SegmentConstraints() []string {
	names := make([]string, 0, len(c.segmentConstraints))
	for name := range c.segmentConstraints {
		names = append(names, name)
	}
	return names
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

// AddSegmentFSConstraint will add or overwrite a constraint function with a given name. A constraint function should return true
// if the given position satisfies the constraint.
func (c *ConstraintChecker) AddSegmentFSConstraint(name string, cons SegmentFSConstraint) {
	if c.segmentFSConstraints == nil {
		c.segmentFSConstraints = map[string]SegmentFSConstraint{}
	}
	name = name + "_" + fmt.Sprintf("%p", cons)
	c.segmentFSConstraints[name] = cons
}

// SegmentFSConstraints will list all FS segment constraints by name.
func (c *ConstraintChecker) SegmentFSConstraints() []string {
	names := make([]string, 0, len(c.segmentFSConstraints))
	for name := range c.segmentFSConstraints {
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

// CheckSegmentAndStateValidityFS will check a segment input and confirm that it 1) meets all segment constraints, and 2) meets all
// state constraints across the segment at some resolution. If it fails an intermediate state, it will return the shortest valid segment,
// provided that segment also meets segment constraints.
func (c *ConstraintChecker) CheckSegmentAndStateValidityFS(
	ctx context.Context,
	segment *SegmentFS,
	resolution float64,
) (*SegmentFS, error) {
	ctx, span := trace.StartSpan(ctx, "CheckSegmentAndStateValidityFS")
	defer span.End()
	subSegment, err := c.CheckStateConstraintsAcrossSegmentFS(ctx, segment, resolution)
	if err != nil {
		if subSegment != nil {
			if c.CheckSegmentFSConstraints(subSegment) == nil {
				return subSegment, err
			}
		}
		return nil, err
	}

	return nil, c.CheckSegmentFSConstraints(segment)
}

// BoundingRegions returns the bounding regions - TODO what does this mean??
func (c *ConstraintChecker) BoundingRegions() []spatialmath.Geometry {
	return c.boundingRegions
}

// PathMetric returns the path metric being used for this ConstraintChecker.
func (c *ConstraintChecker) PathMetric() StateFSMetric {
	return c.pathMetric
}
