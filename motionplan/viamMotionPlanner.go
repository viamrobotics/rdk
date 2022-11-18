package motionplan

import (
	"context"
	"errors"
	"runtime"
	"encoding/json"
	"math/rand"
	"github.com/edaniels/golog"
	
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/utils"
)

// ViamMotionPlanner is intended to be the single entry point to motion planners, wrapping all others, dealing with fallbacks, etc.
// Intended information flow should be:
// motionplan.PlanMotion() -> SolvableFrameSystem.SolveWaypointsWithOptions() -> ViamMotionPlanner.planSingleWaypoint()
type ViamMotionPlanner struct {
	frame *solverFrame
	fs referenceframe.FrameSystem
	logger golog.Logger
	randseed *rand.Rand
}

func NewViamMotionPlanner(frame *solverFrame, fs referenceframe.FrameSystem, logger golog.Logger, seed int) *ViamMotionPlanner {
	return &ViamMotionPlanner{frame, fs, logger, rand.New(rand.NewSource(int64(seed)))}
}

// Plan on the ViamMotionPlanner should be the single point of entry to planning algorithms.
// This struct is responsible for determining what algorithm to use based on the motion requested and user options, calling any fallbacks,
// running pre-checks, etc.
func (mp *ViamMotionPlanner) Plan(
	ctx context.Context,
	goal *commonpb.Pose,
	seed []referenceframe.Input,
	opt *PlannerOptions,
) ([][]referenceframe.Input, error) {
	
	return nil, nil
}

// planSingleWaypoint will solve the solver frame to one individual pose. If you have multiple waypoints to hit, call this multiple times.
func (mp *ViamMotionPlanner) PlanSingleWaypoint(ctx context.Context,
	seedMap map[string][]referenceframe.Input,
	goalPos spatialmath.Pose,
	worldState *commonpb.WorldState,
	motionConfig map[string]interface{},
) ([][]referenceframe.Input, error) {
	seed, err := mp.frame.mapToSlice(seedMap)
	if err != nil {
		return nil, err
	}
	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}
	

	// If we are world rooted, translate the goal pose into the world frame
	if mp.frame.worldRooted {
		tf, err := mp.frame.fss.Transform(seedMap, referenceframe.NewPoseInFrame(mp.frame.goalFrame.Name(), goalPos), referenceframe.World)
		if err != nil {
			return nil, err
		}
		goalPos = tf.(*referenceframe.PoseInFrame).Pose()
	}

	var goals []spatialmath.Pose
	var opts []*PlannerOptions

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := motionConfig["motion_profile"]; ok && profile == LinearMotionProfile {
		pathStepSize, ok := motionConfig["path_step_size"].(float64)
		if !ok {
			pathStepSize = defaultPathStepSize
		}
		numSteps := GetSteps(seedPos, goalPos, pathStepSize)

		from := seedPos
		for i := 1; i < numSteps; i++ {
			by := float64(i) / float64(numSteps)
			to := spatialmath.Interpolate(seedPos, goalPos, by)
			goals = append(goals, to)
			opt, err := mp.plannerSetupFromMoveRequest(from, to, seedMap, worldState, motionConfig)
			if err != nil {
				return nil, err
			}
			opts = append(opts, opt)

			from = to
		}
		seedPos = from
	}
	goals = append(goals, goalPos)
	opt, err := mp.plannerSetupFromMoveRequest(seedPos, goalPos, seedMap, worldState, motionConfig)
	if err != nil {
		return nil, err
	}
	opts = append(opts, opt)

	resultSlices, err := mp.planMotion(ctx, goals, seed, opts, 0)
	if err != nil {
		return nil, err
	}
	return resultSlices, nil
}

// planMotion will plan a single motion, which may be composed of one or more waypoints. Waypoints are here used to begin planning the next
// motion as soon as its starting point is known.
func (mp *ViamMotionPlanner) planMotion(
	ctx context.Context,
	goals []spatialmath.Pose,
	seed []referenceframe.Input,
	opts []*PlannerOptions,
	iter int,
) ([][]referenceframe.Input, error) {
	var err error
	goal := goals[iter]
	opt := opts[iter]
	if opt == nil {
		opt = NewBasicPlannerOptions()
	}
	
	// Build planner
	var pathPlanner MotionPlanner
	if seed, ok := opt.extra["rseed"].(int); ok {
		pathPlanner, err = opt.PlannerConstructor(mp.frame, runtime.NumCPU()/2, rand.New(rand.NewSource(int64(seed))), mp.logger)
	}else{
		pathPlanner, err = opt.PlannerConstructor(mp.frame, runtime.NumCPU()/2, rand.New(rand.NewSource(int64(mp.randseed.Int()))), mp.logger)
	}
	if err != nil {
		return nil, err
	}
	
	remainingSteps := [][]referenceframe.Input{}
	if parPlan, ok := pathPlanner.(RRTParallelPlanner); ok {
		// cBiRRT supports solution look-ahead for parallel waypoint solving
		// TODO(pl): other planners will support lookaheads, so this should be made to be an interface
		endpointPreview := make(chan node, 1)
		solutionChan := make(chan *rrtPlanReturn, 1)
		utils.PanicCapturingGo(func() {
			parPlan.RRTBackgroundRunner(ctx, goal, seed, opt, endpointPreview, solutionChan)
		})
		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			select {
			case nextSeed := <-endpointPreview:
				// Got a solution preview, start solving the next motion in a new thread.
				if iter+1 < len(goals) {
					// In this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it.
					remainingSteps, err = mp.planMotion(ctx, goals, nextSeed.Q(), opts, iter+1)
					if err != nil {
						return nil, err
					}
				}
				for {
					// Get the step from this runner invocation, and return everything in order.
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					select {
					case finalSteps := <-solutionChan:
						if finalSteps.err != nil {
							return nil, finalSteps.err
						}
						results := append(finalSteps.ToInputs(), remainingSteps...)
						return results, nil
					default:
					}
				}
			case finalSteps := <-solutionChan:
				// We didn't get a solution preview (possible error), so we get and process the full step set and error.
				if finalSteps.err != nil {
					return nil, finalSteps.err
				}
				if iter+1 < len(goals) {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it
					remainingSteps, err = mp.planMotion(
						ctx,
						goals,
						finalSteps.steps[len(finalSteps.steps)-1].Q(),
						opts,
						iter+1,
					)
					if err != nil {
						return nil, err
					}
				}
				results := append(finalSteps.ToInputs(), remainingSteps...)
				return results, nil
			default:
			}
		}
	} else {
		resultSlicesRaw, err := pathPlanner.Plan(ctx, goal, seed, opt)
		if err != nil {
			return nil, err
		}
		if iter < len(goals)-2 {
			// in this case, we create the next step (and thus the remaining steps) and the
			// step from our iteration hangs out in the channel buffer until we're done with it
			remainingSteps, err = mp.planMotion(ctx, goals, resultSlicesRaw[len(resultSlicesRaw)-1], opts, iter+1)
			if err != nil {
				return nil, err
			}
		}
		return append(resultSlicesRaw, remainingSteps...), nil
	}
}

func (mp *ViamMotionPlanner) plannerSetupFromMoveRequest(
	from, to spatialmath.Pose,
	seedMap map[string][]referenceframe.Input,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) (*PlannerOptions, error) {
	opt := NewBasicPlannerOptions()
	opt.extra = planningOpts

	collisionConstraint, err := NewCollisionConstraintFromWorldState(mp.frame, mp.fs, worldState, seedMap)
	if err != nil {
		return nil, err
	}
	opt.AddConstraint(defaultCollisionConstraintName, collisionConstraint)

	// error handling around extracting motion_profile information from map[string]interface{}
	var motionProfile string
	profile, ok := planningOpts["motion_profile"]
	if ok {
		motionProfile, ok = profile.(string)
		if !ok {
			return nil, errors.New("could not interpret motion_profile field as string")
		}
	}

	// convert map to json, then to a struct, overwriting present defaults
	jsonString, err := json.Marshal(planningOpts)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, opt)
	if err != nil {
		return nil, err
	}
	
	var planAlg string
	alg, ok := planningOpts["planning_alg"]
	if ok {
		planAlg, ok = alg.(string)
		if !ok {
			return nil, errors.New("could not interpret planning_alg field as string")
		}
		switch planAlg {
		case "cbirrt":
			opt.PlannerConstructor = newCBiRRTMotionPlannerWithSeed
		case "rrtstar":
			opt.PlannerConstructor = newRRTStarConnectMotionPlannerWithSeed
			// no motion profiles for RRT*
			return opt, nil
		default:
			// use default, already set
		}
	}

	switch motionProfile {
	case LinearMotionProfile:
		// Linear constraints
		linTol, ok := planningOpts["line_tolerance"].(float64)
		if !ok {
			// Default
			linTol = defaultLinearDeviation
		}
		orientTol, ok := planningOpts["orient_tolerance"].(float64)
		if !ok {
			// Default
			orientTol = defaultLinearDeviation
		}
		constraint, pathDist := NewAbsoluteLinearInterpolatingConstraint(from, to, linTol, orientTol)
		opt.AddConstraint(defaultLinearConstraintName, constraint)
		opt.pathDist = pathDist
	case PseudolinearMotionProfile:
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultPseudolinearTolerance
		}
		constraint, pathDist := NewProportionalLinearInterpolatingConstraint(from, to, tolerance)
		opt.AddConstraint(defaultPseudolinearConstraintName, constraint)
		opt.pathDist = pathDist
	case OrientationMotionProfile:
		tolerance, ok := planningOpts["tolerance"].(float64)
		if !ok {
			// Default
			tolerance = defaultOrientationDeviation
		}
		constraint, pathDist := NewSlerpOrientationConstraint(from, to, tolerance)
		opt.AddConstraint(defaultOrientationConstraintName, constraint)
		opt.pathDist = pathDist
	case FreeMotionProfile:
		// No restrictions on motion
	default:
		// TODO(pl): once RRT* is workable, use here. Also, update to try pseudolinear first, and fall back to orientation, then to free
		// if unsuccessful
		
		// set up deep copy for fallback
		try1 := deepIshCopyMap(planningOpts)
		try1["motion_profile"] = "linear"
		fbIter, ok := planningOpts["fallback_iter"].(float64)
		if !ok {
			// Default
			fbIter = 200
		}
		
		
		//~ try1["plan_iter"] = fbIter
		//~ try1["timeout"] = 2.0
		//~ try1Opt, err := plannerSetupFromMoveRequest(from, to, f, fs, seedMap, worldState, try1)
		//~ if err != nil {
			//~ return nil, err
		//~ }
		
		try2 := deepIshCopyMap(planningOpts)
		try2["motion_profile"] = "pseudolinear"
		try2["plan_iter"] = fbIter
		try2["timeout"] = 1.0
		try2Opt, err := mp.plannerSetupFromMoveRequest(from, to, seedMap, worldState, try2)
		if err != nil {
			return nil, err
		}
		
		try3 := deepIshCopyMap(planningOpts)
		try3["motion_profile"] = "orientation"
		try3["plan_iter"] = fbIter
		try3["timeout"] = 2.0
		try3Opt, err := mp.plannerSetupFromMoveRequest(from, to, seedMap, worldState, try3)
		if err != nil {
			return nil, err
		}
		
		
		
		//~ try1Opt.Fallback = opt
		//~ try1Opt.Fallback = try2Opt
		try2Opt.Fallback = opt
		//~ try2Opt.Fallback = try3Opt
		try3Opt.Fallback = opt
		opt = try2Opt
	}
	return opt, nil
}
