package armplanning

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// planManager is intended to be the single entry point to motion planners.
type planManager struct {
	pc      *planContext
	request *PlanRequest
	logger  logging.Logger
}

func newPlanManager(ctx context.Context, logger logging.Logger, request *PlanRequest, meta *PlanMeta) (*planManager, error) {
	pc, err := newPlanContext(ctx, logger, request, meta)
	if err != nil {
		return nil, err
	}
	return &planManager{
		pc:      pc,
		logger:  logger,
		request: request,
	}, nil
}

// planMultiWaypoint plans a motion through multiple waypoints, using identical constraints for each
// Any constraints, etc, will be held for the entire motion.
// return trajector (always, even with error), which goal we got to, error.
func (pm *planManager) planMultiWaypoint(ctx context.Context) (motionplan.Trajectory, int, error) {
	ctx, span := trace.StartSpan(ctx, "planMultiWaypoint")
	defer span.End()
	// Theoretically, a plan could be made between two poses, by running IK on both the start and
	// end poses to create sets of seed and goal configurations. However, the blocker here is the
	// lack of a "known good" configuration used to determine which obstacles are allowed to collide
	// with one another.
	if pm.request.StartState.configuration == nil {
		return nil, 0, errors.New("must populate start state configuration if not planning for 2d base/tpspace")
	}

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.request.PlannerOptions.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(pm.request.PlannerOptions.Timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
	}

	traj := motionplan.Trajectory{pm.request.StartState.Configuration()}
	start, err := pm.request.StartState.ComputePoses(ctx, pm.request.FrameSystem)
	if err != nil {
		return nil, 0, err
	}

	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return traj, i, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		to, err := g.ComputePoses(ctx, pm.request.FrameSystem)
		if err != nil {
			return traj, i, err
		}

		pm.logger.Info("planning step", i, "of", len(pm.request.Goals))
		for k, v := range to {
			pm.logger.Debug(k, v)
		}

		if len(g.configuration) > 0 {
			newTraj, err := pm.planToDirectJoints(ctx, traj[len(traj)-1], g)
			if err != nil {
				return traj, i, err
			}
			traj = append(traj, newTraj...)
		} else {
			subGoals, err := pm.generateWaypoints(ctx, start, to)
			if err != nil {
				return traj, i, err
			}

			if len(subGoals) > 1 {
				pm.logger.Infof("\t generateWaypoint turned into %d subGoals", len(subGoals))
			}

			for subGoalIdx, sg := range subGoals {
				singleGoalStart := time.Now()
				newTraj, err := pm.planSingleGoal(ctx, traj[len(traj)-1], sg)
				if err != nil {
					return traj, i, err
				}
				pm.logger.Debugf("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				traj = append(traj, newTraj...)
			}
		}
		start = to
	}

	return traj, len(pm.request.Goals), nil
}

func (pm *planManager) planToDirectJoints(
	ctx context.Context,
	start referenceframe.FrameSystemInputs,
	goal *PlanState,
) ([]referenceframe.FrameSystemInputs, error) {
	ctx, span := trace.StartSpan(ctx, "planToDirectJoints")
	defer span.End()
	fullConfig := referenceframe.FrameSystemInputs{}
	for k, v := range goal.configuration {
		fullConfig[k] = v
	}

	for k, v := range start {
		if len(fullConfig[k]) == 0 {
			fullConfig[k] = v
		}
	}

	goalPoses, err := goal.ComputePoses(ctx, pm.pc.fs)
	if err != nil {
		return nil, err
	}

	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goalPoses)
	if err != nil {
		return nil, err
	}

	err = psc.checkPath(ctx, start, fullConfig)
	if err == nil {
		return []referenceframe.FrameSystemInputs{fullConfig}, nil
	}
	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)

	err = psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: fullConfig,
		FS:            psc.pc.fs,
	})
	if err != nil {
		return nil, fmt.Errorf("want to go to specific joint config but it is invalid: %w", err)
	}

	if false { // true cartesian half
		// TODO(eliot): finish me
		startPoses, err := start.ComputePoses(pm.pc.fs)
		if err != nil {
			return nil, err
		}

		mid := interp(startPoses, goalPoses, .5)

		pm.logger.Infof("foo things\n\t%v\n\t%v\n\t%v", startPoses, mid, goalPoses)

		err = pm.foo(ctx, start, mid)
		if err != nil {
			pm.logger.Infof("foo failed: %v", err)
		} else {
			panic(2)
		}

		// panic(1)
	}

	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc)
	if err != nil {
		return nil, err
	}

	maps := rrtMaps{}
	maps.startMap = rrtMap{&node{inputs: start}: nil}
	maps.goalMap = rrtMap{&node{inputs: fullConfig}: nil}
	maps.optNode = &node{inputs: fullConfig}

	finalSteps, err := pathPlanner.rrtRunner(ctx, &maps)
	if err != nil {
		return nil, err
	}
	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
	return finalSteps.steps, nil
}

func (pm *planManager) planSingleGoal(
	ctx context.Context,
	start referenceframe.FrameSystemInputs,
	goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemInputs, error) {
	ctx, span := trace.StartSpan(ctx, "planSingleGoal")
	defer span.End()
	pm.logger.Debug("start configuration", start)
	pm.logger.Debug("going to", goal)

	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return nil, err
	}

	planSeed, err := initRRTSolutions(ctx, psc)
	if err != nil {
		return nil, err
	}

	pm.logger.Debugf("initRRTSolutions goalMap size: %d", len(planSeed.maps.goalMap))

	if planSeed.steps != nil {
		pm.logger.Debugf("found an ideal ik solution")
		return planSeed.steps, nil
	}

	quickReroute, err := pm.quickReroute(ctx, psc, planSeed.maps.optNode.inputs)
	if err != nil {
		return nil, err
	}
	if quickReroute != nil {
		pm.logger.Debugf("found a quickReroute")
		return quickReroute, nil
	}
	
	pathPlanner, err := newCBiRRTMotionPlanner(ctx, pm.pc, psc)
	if err != nil {
		return nil, err
	}

	finalSteps, err := pathPlanner.rrtRunner(ctx, planSeed.maps)
	if err != nil {
		return nil, err
	}
	finalSteps.steps = smoothPath(ctx, psc, finalSteps.steps)
	return finalSteps.steps, nil
}

// generateWaypoints will return the list of atomic waypoints that correspond to a specific goal in a plan request.
func (pm *planManager) generateWaypoints(ctx context.Context, start, goal referenceframe.FrameSystemPoses,
) ([]referenceframe.FrameSystemPoses, error) {
	_, span := trace.StartSpan(ctx, "generateWaypoints")
	defer span.End()
	if len(pm.request.Constraints.GetLinearConstraint()) == 0 {
		return []referenceframe.FrameSystemPoses{goal}, nil
	}

	stepSize := pm.request.PlannerOptions.PathStepSize

	numSteps := 0
	for frame, pif := range goal {
		steps := motionplan.CalculateStepCount(start[frame].Pose(), pif.Pose(), stepSize)
		if steps > numSteps {
			numSteps = steps
		}
	}

	pm.logger.Debugf("numSteps: %d", numSteps)

	waypoints := []referenceframe.FrameSystemPoses{}

	from := start

	for i := 1; i <= numSteps; i++ {
		by := float64(i) / float64(numSteps)
		to := referenceframe.FrameSystemPoses{}

		for frameName, pif := range goal {
			if from[frameName].Parent() != pif.Parent() {
				return nil, fmt.Errorf("frame mismatch %v %v", from[frameName].Parent(), pif.Parent())
			}
			toPose := spatialmath.Interpolate(from[frameName].Pose(), pif.Pose(), by)
			to[frameName] = referenceframe.NewPoseInFrame(pif.Parent(), toPose)
		}

		waypoints = append(waypoints, to)

		from = to
	}

	return waypoints, nil
}

type rrtMap map[*node]*node

type rrtSolution struct {
	steps []referenceframe.FrameSystemInputs
	maps  *rrtMaps
}

type rrtMaps struct {
	startMap rrtMap
	goalMap  rrtMap
	optNode  *node // The highest quality IK solution
}

// initRRTsolutions will create the maps to be used by a RRT-based algorithm. It will generate IK
// solutions to pre-populate the goal map, and will check if any of those goals are able to be
// directly interpolated to.
func initRRTSolutions(ctx context.Context, psc *planSegmentContext) (*rrtSolution, error) {
	ctx, span := trace.StartSpan(ctx, "initRRTSolutions")
	defer span.End()
	rrt := &rrtSolution{
		maps: &rrtMaps{
			startMap: rrtMap{},
			goalMap:  rrtMap{},
		},
	}

	seed := newConfigurationNode(psc.start)
	// goalNodes are sorted from lowest cost to highest.
	goalNodes, err := getSolutions(ctx, psc)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]

	psc.pc.logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)

	// `defaultOptimalityMultiple` is > 1.0
	reasonableCost := goalNodes[0].cost * defaultOptimalityMultiple
	for _, solution := range goalNodes {
		if solution.checkPath && solution.cost < reasonableCost {
			// If we've already checked the path of a solution that is "reasonable", we can just
			// return now. Otherwise, continue to initialize goal map with keys.
			rrt.steps = []referenceframe.FrameSystemInputs{solution.inputs}
			return rrt, nil
		}

		rrt.maps.goalMap[&node{inputs: solution.inputs}] = nil
	}
	rrt.maps.startMap[&node{inputs: seed.inputs}] = nil

	return rrt, nil
}

func interp(start, end referenceframe.FrameSystemPoses, delta float64) referenceframe.FrameSystemPoses {
	mid := referenceframe.FrameSystemPoses{}

	for k, s := range start {
		e, ok := end[k]
		if !ok {
			mid[k] = s
			continue
		}
		if s.Parent() != e.Parent() {
			panic("eliottttt")
		}
		m := spatialmath.Interpolate(s.Pose(), e.Pose(), delta)
		mid[k] = referenceframe.NewPoseInFrame(s.Parent(), m)
	}
	return mid
}

func (pm *planManager) quickReroute(ctx context.Context, psc *planSegmentContext, goal referenceframe.FrameSystemInputs) ([]referenceframe.FrameSystemInputs, error) {

	goalPoses, err := goal.ComputePoses(pm.pc.fs)
	if err != nil {
		return nil, err
	}
	
	mid := interp(psc.startPoses, goalPoses, .5)

	offsetDistance := 100.0 // TODO
	numPoints := 6

	for f, s := range psc.startPoses {
		m := mid[f]
		pm.logger.Infof("%v -> %v", s, m)

		// Compute vector from s to m
		sPoint := s.Pose().Point()
		mPoint := m.Pose().Point()
		vec := mPoint.Sub(sPoint)

		// Compute a perpendicular vector using cross product with a reference vector
		// Use the Z-axis as reference (0, 0, 1) unless vec is parallel to it
		var perpVec r3.Vector
		if vec.X != 0 || vec.Y != 0 {
			// Cross product with Z-axis: (vec.Y, -vec.X, 0)
			perpVec = r3.Vector{X: vec.Y, Y: -vec.X, Z: 0}
		} else {
			// vec is parallel to Z-axis, use X-axis instead
			perpVec = r3.Vector{X: 0, Y: vec.Z, Z: -vec.Y}
		}

		// Normalize the perpendicular vector
		perpUnit := perpVec.Normalize()

		// Create 8 points around a circle perpendicular to the vec
		angleStep := 2 * math.Pi / float64(numPoints)
		for i := 0; i < numPoints; i++ {
			angle := float64(i) * angleStep

			// Rotate perpUnit around vec axis by angle
			// Using Rodrigues' rotation formula: v_rot = v*cos(θ) + (k × v)*sin(θ) + k*(k·v)*(1-cos(θ))
			// where k is the unit vector along vec
			k := vec.Normalize()
			cosTheta := math.Cos(angle)
			sinTheta := math.Sin(angle)

			// k × perpUnit
			kCrossPerpUnit := k.Cross(perpUnit)

			// k · perpUnit (should be 0 since perpUnit is perpendicular to vec)
			kDotPerpUnit := k.Dot(perpUnit)

			// Rotated vector
			rotated := perpUnit.Mul(cosTheta).
				Add(kCrossPerpUnit.Mul(sinTheta)).
				Add(k.Mul(kDotPerpUnit * (1 - cosTheta)))

			// Offset from m
			offsetPoint := mPoint.Add(rotated.Mul(offsetDistance))

			pm.logger.Infof("circle point %d: %v", i, offsetPoint)
		}
	}
	
	return nil, fmt.Errorf("finish me %v", mid)
}

func (pm *planManager) foo(ctx context.Context, start referenceframe.FrameSystemInputs, goal referenceframe.FrameSystemPoses) error {
	psc, err := newPlanSegmentContext(ctx, pm.pc, start, goal)
	if err != nil {
		return err
	}

	planSeed, err := initRRTSolutions(ctx, psc)
	if err != nil {
		return err
	}

	if planSeed.steps == nil {
		return fmt.Errorf("no steps")
	}

	if len(planSeed.steps) != 1 {
		return fmt.Errorf("steps odd %d", len(planSeed.steps))
	}

	err = psc.checkPath(ctx, start, planSeed.steps[0])
	if err != nil {
		return err
	}

	panic(5)
}
