package armplanning

import (
	"context"
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
func (pm *planManager) planMultiWaypoint(ctx context.Context) ([]*referenceframe.LinearInputs, int, error) {
	ctx, span := trace.StartSpan(ctx, "planMultiWaypoint")
	defer span.End()

	// set timeout for entire planning process if specified
	var cancel func()
	if pm.request.PlannerOptions.Timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, pm.request.PlannerOptions.timeoutDuration())
	}
	if cancel != nil {
		defer cancel()
	}

	linearTraj := []*referenceframe.LinearInputs{pm.request.StartState.LinearConfiguration()}
	start, err := pm.request.StartState.ComputePoses(ctx, pm.request.FrameSystem)
	if err != nil {
		return nil, 0, err
	}

	for i, g := range pm.request.Goals {
		if ctx.Err() != nil {
			return linearTraj, i, err // note: here and below, we return traj because of ReturnPartialPlan
		}

		to, err := g.ComputePoses(ctx, pm.request.FrameSystem)
		if err != nil {
			return linearTraj, i, err
		}

		pm.logger.Info("planning step", i, "of", len(pm.request.Goals))
		for k, v := range to {
			pm.logger.Debug(k, v)
		}

		if len(g.Configuration()) > 0 {
			newTraj, err := pm.planToDirectJoints(ctx, linearTraj[len(linearTraj)-1], g)
			if err != nil {
				return linearTraj, i, err
			}
			linearTraj = append(linearTraj, newTraj...)
		} else {
			subGoals, err := pm.generateWaypoints(ctx, start, to)
			if err != nil {
				return linearTraj, i, err
			}

			if len(subGoals) > 1 {
				pm.logger.Infof("\t generateWaypoint turned into %d subGoals", len(subGoals))
			}

			for subGoalIdx, sg := range subGoals {
				singleGoalStart := time.Now()
				newTraj, err := pm.planSingleGoal(ctx, linearTraj[len(linearTraj)-1], sg)
				if err != nil {
					return linearTraj, i, err
				}
				pm.logger.Debugf("\t subgoal %d took %v", subGoalIdx, time.Since(singleGoalStart))
				linearTraj = append(linearTraj, newTraj...)
			}
		}
		start = to
	}

	return linearTraj, len(pm.request.Goals), nil
}

func (pm *planManager) planToDirectJoints(
	ctx context.Context,
	start *referenceframe.LinearInputs,
	goal *PlanState,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "planToDirectJoints")
	defer span.End()
	fullConfig := referenceframe.NewLinearInputs()
	for k, v := range goal.Configuration() {
		fullConfig.Put(k, v)
	}

	for k, v := range start.Items() {
		if len(fullConfig.Get(k)) == 0 {
			fullConfig.Put(k, v)
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
		return []*referenceframe.LinearInputs{fullConfig}, nil
	}

	pm.logger.Debugf("want to go to specific joint positions, but path is blocked: %v", err)
	err = psc.checker.CheckStateFSConstraints(ctx, &motionplan.StateFS{
		Configuration: fullConfig,
		FS:            psc.pc.fs,
	})
	if err != nil {
		return nil, fmt.Errorf("want to go to specific joint config but it is invalid: %w", err)
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
	start *referenceframe.LinearInputs,
	goal referenceframe.FrameSystemPoses,
) ([]*referenceframe.LinearInputs, error) {
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

	if false {
		quickReroute, err := pm.quickReroute(ctx, psc, planSeed.maps.optNode.inputs)
		if err != nil {
			return nil, err
		}
		if quickReroute != nil {
			pm.logger.Debugf("found a quickReroute")
			return quickReroute, nil
		}
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

	tighestConstraint := 10.0

	for _, lc := range pm.request.Constraints.GetLinearConstraint() {
		tighestConstraint = min(tighestConstraint, lc.LineToleranceMm)
		tighestConstraint = min(tighestConstraint, lc.OrientationToleranceDegs)
	}

	tighestConstraint = max(tighestConstraint, 0)

	stepSize := defaultStepSizeMM / max(1, ((10-tighestConstraint)/2))
	pm.logger.Debugf("stepSize: %0.2f", stepSize)

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
	steps []*referenceframe.LinearInputs
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
	goalNodes, err := getSolutions(ctx, psc, nil)
	if err != nil {
		return rrt, err
	}

	rrt.maps.optNode = goalNodes[0]

	psc.pc.logger.Debugf("optNode cost: %v", rrt.maps.optNode.cost)

	// `defaultOptimalityMultiple` is > 1.0
	reasonableCost := max(.01, goalNodes[0].cost) * defaultOptimalityMultiple
	for _, solution := range goalNodes {
		if solution.cost > reasonableCost {
			// if it's this bad, we don't want for cbirrt or going straight
			continue
		}

		if solution.checkPath {
			// If we've already checked the path of a solution that is "reasonable", we can just
			// return now. Otherwise, continue to initialize goal map with keys.
			rrt.steps = []*referenceframe.LinearInputs{solution.inputs}
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

func (pm *planManager) quickReroute(ctx context.Context,
	psc *planSegmentContext,
	goal *referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "quickReroute")
	defer span.End()

	mid := interp(psc.startPoses, psc.goal, .5)

	pm.logger.Infof("quickReroute\n\tstart: %v\n\tmid: %v\n\tgoal: %v", psc.startPoses, mid, psc.goal)

	numPoints := 4

	angleStep := 2 * math.Pi / float64(numPoints)

	// In the event a straight line path is not achievable due to an obstacle, we want to assume
	// there's a _single_ (convex hull of an) obstacle. Such that we can aim for the side of our
	// goal to work our way around.
	//
	// This algorithm takes the midpoint(s) between each start position and goal. And for each start
	// position <-> goal pair:
	//
	// 1. Identifies the midpoint and creates a vector from the start to the midpoint.
	// 2. Computes a vector normal/perpendicular to the above one.
	// 3. Create new "side-quest" goal candidates by:
	// 4.  Creating four points (`i` -> `angle`) in a circle by rotating the normal vector around.
	// 5.  Creating a larger normal vector/radius (`j` -> `myDistance`) and repeating the above
	//     rotation.
	for j := .5; j <= 2; j += .5 {
		for i := 0; i < numPoints; i++ {
			angle := float64(i) * angleStep
			attempt := referenceframe.FrameSystemPoses{}

			offsetDistance := 0.0

			for f, s := range psc.startPoses {
				m := mid[f]

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

				perpUnit := perpVec.Normalize() // Normalize the perpendicular vector

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

				myDistance := j * sPoint.Distance(mPoint)
				offsetDistance += myDistance
				offsetPoint := mPoint.Add(rotated.Mul(myDistance))

				attempt[f] = referenceframe.NewPoseInFrame(
					s.Parent(),
					spatialmath.NewPose(offsetPoint, s.Pose().Orientation())) // what to do with orientation
			}

			pm.logger.Infof("circle point %v", attempt)

			// Now that we've generated a bunch of new goal candidates (`attempt`), run IK and see
			// if anything shakes out.
			psc2, err := newPlanSegmentContext(ctx, pm.pc, psc.start, attempt)
			if err != nil {
				return nil, err
			}

			sol, err := getSolutions(ctx, psc2, psc.pc.linearizeFSmetric(quickRerouteGoalMetric(psc2.goal, offsetDistance/4)))
			if err != nil {
				pm.logger.Debugf("attempt failed: %v", err)
				continue
			}

			for _, s := range sol {
				pm.logger.Infof(" sol %v", s)

				if !s.checkPath {
					continue
				}

				err = psc2.checkPath(ctx, s.inputs, goal)
				pm.logger.Infof(" sol %v -> %v", s, err)
				if err == nil {
					return []*referenceframe.LinearInputs{s.inputs, goal}, nil
				}
			}
		}
	}

	return nil, nil
}

func quickRerouteGoalMetric(goal referenceframe.FrameSystemPoses, delta float64) motionplan.StateFSMetric {
	return func(state *motionplan.StateFS) float64 {
		score := 0.
		for frame, goalInFrame := range goal {
			currPose, err := state.FS.Transform(state.Configuration, referenceframe.NewZeroPoseInFrame(frame), goalInFrame.Parent())
			if err != nil {
				panic(fmt.Errorf("fs: %v err: %w frame: %s poseParent: %v", state.FS.FrameNames(), err, frame, goalInFrame.Parent()))
			}

			myScoore := currPose.(*referenceframe.PoseInFrame).Pose().Point().Distance(goal[frame].Pose().Point())
			score += max(0, myScoore-delta)
		}
		return score
	}
}
