package motionplan

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/edaniels/golog"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	defaultOptimalityMultiple = 2.0
	defaultFallbackTimeout    = 1.5
)

// planManager is intended to be the single entry point to motion planners, wrapping all others, dealing with fallbacks, etc.
// Intended information flow should be:
// motionplan.PlanMotion() -> SolvableFrameSystem.SolveWaypointsWithOptions() -> planManager.planSingleWaypoint().
type planManager struct {
	*planner
	frame *solverFrame
	fs    referenceframe.FrameSystem
}

func newPlanManager(frame *solverFrame, fs referenceframe.FrameSystem, logger golog.Logger, seed int) (*planManager, error) {
	//nolint: gosec
	p, err := newPlanner(frame, runtime.NumCPU()/2, rand.New(rand.NewSource(int64(seed))), logger, nil)
	if err != nil {
		return nil, err
	}
	return &planManager{p, frame, fs}, nil
}

// PlanSingleWaypoint will solve the solver frame to one individual pose. If you have multiple waypoints to hit, call this multiple times.
// Any constraints, etc, will be held for the entire motion.
func (mp *planManager) PlanSingleWaypoint(ctx context.Context,
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

	var cancel func()

	// set timeout for entire planning process if specified
	if timeout, ok := motionConfig["timeout"].(float64); ok {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	if cancel != nil {
		defer cancel()
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
	var opts []*plannerOptions

	// linear motion profile has known intermediate points, so solving can be broken up and sped up
	if profile, ok := motionConfig["motion_profile"]; ok && profile == LinearMotionProfile {
		pathStepSize, ok := motionConfig["path_step_size"].(float64)
		if !ok {
			pathStepSize = defaultPathStepSize
		}
		numSteps := PathStepCount(seedPos, goalPos, pathStepSize)

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

	resultSlices, err := mp.planMotion(ctx, goals, seed, opts, nil, 0)
	if err != nil {
		return nil, err
	}
	return resultSlices, nil
}

// planMotion will plan a single motion, which may be composed of one or more waypoints. Waypoints are here used to begin planning the next
// motion as soon as its starting point is known.
func (mp *planManager) planMotion(
	ctx context.Context,
	goals []spatialmath.Pose,
	seed []referenceframe.Input,
	opts []*plannerOptions,
	rm *rrtMaps,
	iter int,
) ([][]referenceframe.Input, error) {
	var err error
	goal := goals[iter]
	opt := opts[iter]
	if opt == nil {
		opt = newBasicPlannerOptions()
	}

	// Build planner
	var pathPlanner motionPlanner
	if seed, ok := opt.extra["rseed"].(int); ok {
		//nolint: gosec
		pathPlanner, err = opt.PlannerConstructor(
			mp.frame,
			runtime.NumCPU()/2,
			rand.New(rand.NewSource(int64(seed))),
			mp.logger,
			opt,
		)
	} else {
		//nolint: gosec
		pathPlanner, err = opt.PlannerConstructor(
			mp.frame,
			runtime.NumCPU()/2,
			rand.New(rand.NewSource(int64(mp.randseed.Int()))),
			mp.logger,
			opt,
		)
	}
	if err != nil {
		return nil, err
	}
	remainingSteps := [][]referenceframe.Input{}

	// If we don't pass in pre-made maps, initialize and seed with IK solutions here
	if rm == nil {
		planSeed := initRRTMaps(ctx, pathPlanner, goal, seed)
		if planSeed.planerr != nil {
			return nil, planSeed.planerr
		}
		if planSeed.steps != nil {
			if iter+1 < len(goals) {
				// in this case, we create the next step (and thus the remaining steps) and the
				// step from our iteration hangs out in the buffer until we're done with it
				remainingSteps, err = mp.planMotion(
					ctx,
					goals,
					planSeed.steps[len(planSeed.steps)-1].Q(),
					opts,
					nil,
					iter+1,
				)
				if err != nil {
					return nil, err
				}
			}
			results := append(planSeed.toInputs(), remainingSteps...)
			return results, nil
		}

		rm = planSeed.rm
	}

	planctx, cancel := context.WithTimeout(ctx, time.Duration(opt.Timeout*float64(time.Second)))
	defer cancel()

	if parPlan, ok := pathPlanner.(rrtParallelPlanner); ok {
		// publish endpoint of plan if it is known
		var nextSeed node
		if len(rm.goalMap) == 1 {
			mp.logger.Debug("found early final solution")
			for key := range rm.goalMap {
				nextSeed = key
			}
		}
		// rrtParallelPlanner supports solution look-ahead for parallel waypoint solving
		endpointPreview := make(chan node, 1)
		solutionChan := make(chan *rrtPlanReturn, 1)
		utils.PanicCapturingGo(func() {
			if nextSeed == nil {
				parPlan.rrtBackgroundRunner(planctx, goal, seed, &rrtParallelPlannerShared{rm, endpointPreview, solutionChan})
			} else {
				endpointPreview <- nextSeed
				parPlan.rrtBackgroundRunner(planctx, goal, seed, &rrtParallelPlannerShared{rm, nil, solutionChan})
			}
		})

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			select {
			case nextSeed = <-endpointPreview:
				// Got a solution preview, start solving the next motion in a new thread.
				if iter+1 < len(goals) {
					// In this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the channel buffer until we're done with it.
					remainingSteps, err = mp.planMotion(ctx, goals, nextSeed.Q(), opts, nil, iter+1)
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
						if finalSteps.err() != nil {
							return nil, finalSteps.err()
						}
						results := append(finalSteps.toInputs(), remainingSteps...)
						return results, nil
					default:
					}
				}
			case finalSteps := <-solutionChan:
				// We didn't get a solution preview (possible error), so we get and process the full step set and error.

				nextSeed := finalSteps.rm

				// default to fallback; will unset if we have a good path
				goodSolution := false

				// If there was no error, check path quality. If sufficiently good, move on
				if finalSteps.err() == nil {
					if opt.Fallback != nil {
						if ok, score := goodPlan(finalSteps, opt); ok {
							mp.logger.Debugf("got path with score %f, close enough to optimal %f", score, rm.optNode.cost)
							goodSolution = true
						} else {
							mp.logger.Debugf("path with score %f not close enough to optimal %f, falling back", score, rm.optNode.cost)

							// If we have a connected but bad path, we recreate new IK solutions and start from scratch
							// rather than seeding with a completed, known-bad tree
							nextSeed = nil
							opt.Fallback.MaxSolutions = opt.MaxSolutions
						}
					} else {
						goodSolution = true
					}
				}

				// Run fallback if necessary. This will also run smoothing if our score isn't good enough.
				if !goodSolution {
					smoothChan := make(chan []node, 1)
					utils.PanicCapturingGo(func() {
						smoothChan <- parPlan.smoothPath(ctx, finalSteps.steps)
					})
					if opt.Fallback != nil {
						alternate, err := mp.planMotion(
							ctx,
							[]spatialmath.Pose{goal},
							seed,
							[]*plannerOptions{opt.Fallback},
							nextSeed,
							iter,
						)
						finalSteps.steps = <-smoothChan
						_, score := goodPlan(finalSteps, opt)
						if err == nil {
							altCost := EvaluatePlan(&rrtPlanReturn{steps: stepsToNodes(alternate)}, opt)
							if altCost < score {
								mp.logger.Debugf("replacing path with score %f with better score %f", score, altCost)
								finalSteps = &rrtPlanReturn{steps: stepsToNodes(alternate)}
							} else {
								mp.logger.Debugf("fallback path with score %f worse than original score %f; using original", altCost, score)
							}
						}
					} else {
						finalSteps.steps = <-smoothChan
					}
				}

				if finalSteps.err() != nil {
					return nil, finalSteps.err()
				}

				if iter+1 < len(goals) {
					// in this case, we create the next step (and thus the remaining steps) and the
					// step from our iteration hangs out in the buffer until we're done with it
					remainingSteps, err = mp.planMotion(
						ctx,
						goals,
						finalSteps.steps[len(finalSteps.steps)-1].Q(),
						opts,
						nil,
						iter+1,
					)
					if err != nil {
						return nil, err
					}
				}
				results := append(finalSteps.toInputs(), remainingSteps...)
				return results, nil
			default:
			}
		}
	} else {
		resultSlicesRaw, err := pathPlanner.Plan(planctx, goal, seed)
		if err != nil {
			return nil, err
		}
		if iter < len(goals)-2 {
			// in this case, we create the next step (and thus the remaining steps)
			remainingSteps, err = mp.planMotion(ctx, goals, resultSlicesRaw[len(resultSlicesRaw)-1], opts, nil, iter+1)
			if err != nil {
				return nil, err
			}
		}
		return append(resultSlicesRaw, remainingSteps...), nil
	}
}

// This is where the map[string]interface{} passed in via `extra` is used to decide how planning happens.
func (mp *planManager) plannerSetupFromMoveRequest(
	from, to spatialmath.Pose,
	seedMap map[string][]referenceframe.Input,
	worldState *commonpb.WorldState,
	planningOpts map[string]interface{},
) (*plannerOptions, error) {
	// Start with normal options
	opt := newBasicPlannerOptions()

	opt.extra = planningOpts

	// set this to true to get collision penetration depth
	// not yet fully supported, but could be used by cbirrt
	getColDepth := false

	collisionConstraint, err := NewCollisionConstraintFromWorldState(mp.frame, mp.fs, worldState, seedMap, getColDepth)
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
		// TODO(pl): make these consts
		case "cbirrt":
			opt.PlannerConstructor = newCBiRRTMotionPlanner
		case "rrtstar":
			// no motion profiles for RRT*
			opt.PlannerConstructor = newRRTStarConnectMotionPlanner
			// TODO(pl): more logic for RRT*?
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
	case PositionOnlyMotionProfile:
		opt.SetMetric(NewPositionOnlyMetric())
	case FreeMotionProfile:
		// No restrictions on motion
		fallthrough
	default:
		if planAlg == "" {
			// set up deep copy for fallback
			try1 := deepAtomicCopyMap(planningOpts)
			// No need to generate tons more IK solutions when the first alg will do it

			// time to run the first planning attempt before falling back
			try1["timeout"] = defaultFallbackTimeout
			try1["planning_alg"] = "rrtstar"
			try1Opt, err := mp.plannerSetupFromMoveRequest(from, to, seedMap, worldState, try1)
			if err != nil {
				return nil, err
			}

			try1Opt.Fallback = opt
			opt = try1Opt
		}
	}
	return opt, nil
}

// check whether the solution is within some amount of the optimal.
func goodPlan(pr *rrtPlanReturn, opt *plannerOptions) (bool, float64) {
	solutionCost := math.Inf(1)
	if pr.steps != nil {
		if pr.rm.optNode.cost <= 0 {
			return true, solutionCost
		}
		solutionCost = EvaluatePlan(pr, opt)
		if solutionCost < pr.rm.optNode.cost*defaultOptimalityMultiple {
			return true, solutionCost
		}
	}

	return false, solutionCost
}

// Copy any atomic values.
func deepAtomicCopyMap(opt map[string]interface{}) map[string]interface{} {
	optCopy := map[string]interface{}{}
	for k, v := range opt {
		optCopy[k] = v
	}
	return optCopy
}
