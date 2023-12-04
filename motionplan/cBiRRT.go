//go:build !windows && !no_cgo

package motionplan

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	// Maximum number of iterations that constrainNear will run before exiting nil.
	// Typically it will solve in the first five iterations, or not at all.
	maxNearIter = 20

	// Maximum number of iterations that constrainedExtend will run before exiting.
	maxExtendIter = 5000

	nloptJump = 0.00000001
)

type cbirrtOptions struct {
	// Number of IK solutions with which to seed the goal side of the bidirectional tree.
	SolutionsToSeed int `json:"solutions_to_seed"`
}

// newCbirrtOptions creates a struct controlling the running of a single invocation of cbirrt. All values are pre-set to reasonable
// defaults, but can be tweaked if needed.
func newCbirrtOptions(planOpts *plannerOptions) (*cbirrtOptions, error) {
	algOpts := &cbirrtOptions{
		SolutionsToSeed: defaultSolutionsToSeed,
	}
	// convert map to json
	jsonString, err := json.Marshal(planOpts.extra)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonString, algOpts)
	if err != nil {
		return nil, err
	}

	return algOpts, nil
}

// cBiRRTMotionPlanner an object able to solve constrained paths around obstacles to some goal for a given referenceframe.
// It uses the Constrained Bidirctional Rapidly-expanding Random Tree algorithm, Berenson et al 2009
// https://ieeexplore.ieee.org/document/5152399/
type cBiRRTMotionPlanner struct {
	*planner
	fastGradDescent *ik.NloptIK
	algOpts         *cbirrtOptions
}

// newCBiRRTMotionPlannerWithSeed creates a cBiRRTMotionPlanner object with a user specified random seed.
func newCBiRRTMotionPlanner(
	frame referenceframe.Frame,
	seed *rand.Rand,
	logger logging.Logger,
	opt *plannerOptions,
) (motionPlanner, error) {
	if opt == nil {
		return nil, errNoPlannerOptions
	}
	mp, err := newPlanner(frame, seed, logger, opt)
	if err != nil {
		return nil, err
	}
	// nlopt should try only once
	nlopt, err := ik.CreateNloptIKSolver(frame, logger, 1, true, nloptJump)
	if err != nil {
		return nil, err
	}
	algOpts, err := newCbirrtOptions(opt)
	if err != nil {
		return nil, err
	}
	return &cBiRRTMotionPlanner{
		planner:         mp,
		fastGradDescent: nlopt,
		algOpts:         algOpts,
	}, nil
}

func (mp *cBiRRTMotionPlanner) plan(ctx context.Context,
	goal spatialmath.Pose,
	seed []referenceframe.Input,
) ([]node, error) {
	mp.planOpts.SetGoal(goal)
	solutionChan := make(chan *rrtPlanReturn, 1)
	utils.PanicCapturingGo(func() {
		mp.rrtBackgroundRunner(ctx, seed, &rrtParallelPlannerShared{nil, nil, solutionChan})
	})
	plan := <-solutionChan
	return plan.steps, plan.err()
}

// rrtBackgroundRunner will execute the plan. Plan() will call rrtBackgroundRunner in a separate thread and wait for results.
// Separating this allows other things to call rrtBackgroundRunner in parallel allowing the thread-agnostic Plan to be accessible.
func (mp *cBiRRTMotionPlanner) rrtBackgroundRunner(
	ctx context.Context,
	seed []referenceframe.Input,
	rrt *rrtParallelPlannerShared,
) {
	defer close(rrt.solutionChan)

	// setup planner options
	if mp.planOpts == nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: errNoPlannerOptions}
		return
	}
	// initialize maps
	// TODO(rb) package neighborManager better
	nm1 := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nm2 := &neighborManager{nCPU: mp.planOpts.NumThreads}
	nmContext, cancel := context.WithCancel(ctx)
	defer cancel()
	mp.start = time.Now()

	if rrt.maps == nil || len(rrt.maps.goalMap) == 0 {
		planSeed := initRRTSolutions(ctx, mp, seed)
		if planSeed.planerr != nil || planSeed.steps != nil {
			rrt.solutionChan <- planSeed
			return
		}
		rrt.maps = planSeed.maps
	}
	mp.logger.Infof("goal node: %v\n", rrt.maps.optNode.Q())
	target := newConfigurationNode(referenceframe.InterpolateInputs(seed, rrt.maps.optNode.Q(), 0.5))

	map1, map2 := rrt.maps.startMap, rrt.maps.goalMap

	m1chan := make(chan node, 1)
	m2chan := make(chan node, 1)
	defer close(m1chan)
	defer close(m2chan)

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		rrt.solutionChan <- &rrtPlanReturn{planerr: err}
		return
	}

	mp.logger.Debugf(
		"running CBiRRT from start pose %v with start map of size %d and goal map of size %d",
		spatialmath.PoseToProtobuf(seedPos),
		len(rrt.maps.startMap),
		len(rrt.maps.goalMap),
	)

	for i := 0; i < mp.planOpts.PlanIter; i++ {
		select {
		case <-ctx.Done():
			mp.logger.Debugf("CBiRRT timed out after %d iterations", i)
			rrt.solutionChan <- &rrtPlanReturn{planerr: fmt.Errorf("cbirrt timeout %w", ctx.Err()), maps: rrt.maps}
			return
		default:
		}

		tryExtend := func(target node) (node, node, error) {
			// attempt to extend maps 1 and 2 towards the target
			utils.PanicCapturingGo(func() {
				m1chan <- nm1.nearestNeighbor(nmContext, mp.planOpts, target, map1)
			})
			utils.PanicCapturingGo(func() {
				m2chan <- nm2.nearestNeighbor(nmContext, mp.planOpts, target, map2)
			})
			nearest1 := <-m1chan
			nearest2 := <-m2chan
			// If ctx is done, nearest neighbors will be invalid and we want to return immediately
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}

			//nolint: gosec
			rseed1 := rand.New(rand.NewSource(int64(mp.randseed.Int())))
			//nolint: gosec
			rseed2 := rand.New(rand.NewSource(int64(mp.randseed.Int())))

			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed1, map1, nearest1, target, m1chan)
			})
			utils.PanicCapturingGo(func() {
				mp.constrainedExtend(ctx, rseed2, map2, nearest2, target, m2chan)
			})
			map1reached := <-m1chan
			map2reached := <-m2chan

			map1reached.SetCorner(true)
			map2reached.SetCorner(true)

			return map1reached, map2reached, nil
		}

		map1reached, map2reached, err := tryExtend(target)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}

		reachedDelta := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})

		// Second iteration; extend maps 1 and 2 towards the halfway point between where they reached
		if reachedDelta > mp.planOpts.JointSolveDist {
			target = newConfigurationNode(referenceframe.InterpolateInputs(map1reached.Q(), map2reached.Q(), 0.5))
			map1reached, map2reached, err = tryExtend(target)
			if err != nil {
				rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
				return
			}
			reachedDelta = mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: map1reached.Q(), EndConfiguration: map2reached.Q()})
		}

		// Solved!
		if reachedDelta <= mp.planOpts.JointSolveDist {
			mp.logger.Debugf("CBiRRT found solution after %d iterations", i)
			cancel()
			path := extractPath(rrt.maps.startMap, rrt.maps.goalMap, &nodePair{map1reached, map2reached}, true)
			rrt.solutionChan <- &rrtPlanReturn{steps: path, maps: rrt.maps}
			return
		}

		// sample near map 1 and switch which map is which to keep adding to them even
		target, err = mp.sample(map1reached, i)
		if err != nil {
			rrt.solutionChan <- &rrtPlanReturn{planerr: err, maps: rrt.maps}
			return
		}
		map1, map2 = map2, map1
	}
	rrt.solutionChan <- &rrtPlanReturn{planerr: errPlannerFailed, maps: rrt.maps}
}

// constrainedExtend will try to extend the map towards the target while meeting constraints along the way. It will
// return the closest solution to the target that it reaches, which may or may not actually be the target.
func (mp *cBiRRTMotionPlanner) constrainedExtend(
	ctx context.Context,
	randseed *rand.Rand,
	rrtMap map[node]node,
	near, target node,
	mchan chan node,
) {
	// Allow qstep to be doubled as a means to escape from configurations which gradient descend to their seed
	qstep := make([]float64, len(mp.planOpts.qstep))
	copy(qstep, mp.planOpts.qstep)
	doubled := false

	oldNear := near
	// This should iterate until one of the following conditions:
	// 1) we have reached the target
	// 2) the request is cancelled/times out
	// 3) we are no longer approaching the target and our "best" node is further away than the previous best
	// 4) further iterations change our best node by close-to-zero amounts
	// 5) we have iterated more than maxExtendIter times
	for i := 0; i < maxExtendIter; i++ {
		select {
		case <-ctx.Done():
			mchan <- oldNear
			return
		default:
		}

		dist := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: near.Q(), EndConfiguration: target.Q()})
		oldDist := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: oldNear.Q(), EndConfiguration: target.Q()})
		switch {
		case dist < mp.planOpts.JointSolveDist:
			mchan <- near
			return
		case dist > oldDist:
			mchan <- oldNear
			return
		}

		oldNear = near

		newNear := fixedStepInterpolation(near, target, mp.planOpts.qstep)
		// Check whether newNear meets constraints, and if not, update it to a configuration that does meet constraints (or nil)
		newNear = mp.constrainNear(ctx, randseed, oldNear.Q(), newNear)

		if newNear != nil {
			nearDist := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: oldNear.Q(), EndConfiguration: newNear})
			if nearDist < math.Pow(mp.planOpts.JointSolveDist, 3) {
				if !doubled {
					doubled = true
					// Check if doubling qstep will allow escape from the identical configuration
					// If not, we terminate and return.
					// If so, qstep will be reset to its original value after the rescue.
					for i, q := range qstep {
						qstep[i] = q * 2.0
					}
					continue
				} else {
					// We've arrived back at very nearly the same configuration again; stop solving and send back oldNear.
					// Do not add the near-identical configuration to the RRT map
					mchan <- oldNear
					return
				}
			}
			if doubled {
				copy(qstep, mp.planOpts.qstep)
				doubled = false
			}
			// constrainNear will ensure path between oldNear and newNear satisfies constraints along the way
			near = &basicNode{q: newNear}
			rrtMap[near] = oldNear
		} else {
			break
		}
	}
	mchan <- oldNear
}

// constrainNear will do a IK gradient descent from seedInputs to target. If a gradient descent distance
// function has been specified, this will use that.
// This function will return either a valid configuration that meets constraints, or nil.
func (mp *cBiRRTMotionPlanner) constrainNear(
	ctx context.Context,
	randseed *rand.Rand,
	seedInputs,
	target []referenceframe.Input,
) []referenceframe.Input {
	for i := 0; i < maxNearIter; i++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		seedPos, err := mp.frame.Transform(seedInputs)
		if err != nil {
			return nil
		}
		goalPos, err := mp.frame.Transform(target)
		if err != nil {
			return nil
		}

		newArc := &ik.Segment{
			StartPosition:      seedPos,
			EndPosition:        goalPos,
			StartConfiguration: seedInputs,
			EndConfiguration:   target,
			Frame:              mp.frame,
		}

		// Check if the arc of "seedInputs" to "target" is valid
		ok, _ := mp.planOpts.CheckSegmentAndStateValidity(newArc, mp.planOpts.Resolution)
		if ok {
			return target
		}
		solutionGen := make(chan *ik.Solution, 1)
		// Spawn the IK solver to generate solutions until done
		err = mp.fastGradDescent.Solve(ctx, solutionGen, target, mp.planOpts.pathMetric, randseed.Int())
		// We should have zero or one solutions
		var solved *ik.Solution
		select {
		case solved = <-solutionGen:
		default:
		}
		close(solutionGen)
		if err != nil || solved == nil {
			return nil
		}

		ok, failpos := mp.planOpts.CheckSegmentAndStateValidity(
			&ik.Segment{StartConfiguration: seedInputs, EndConfiguration: solved.Configuration, Frame: mp.frame},
			mp.planOpts.Resolution,
		)
		if ok {
			return solved.Configuration
		}
		if failpos != nil {
			dist := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: target, EndConfiguration: failpos.EndConfiguration})
			if dist > mp.planOpts.JointSolveDist {
				// If we have a first failing position, and that target is updating (no infinite loop), then recurse
				seedInputs = failpos.StartConfiguration
				target = failpos.EndConfiguration
			}
		} else {
			return nil
		}
	}
	return nil
}

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func (mp *cBiRRTMotionPlanner) smoothPath(
	ctx context.Context,
	inputSteps []node,
) []node {
	toIter := int(math.Min(float64(len(inputSteps)*len(inputSteps)), float64(mp.planOpts.SmoothIter)))

	schan := make(chan node, 1)
	defer close(schan)

	for numCornersToPass := 2; numCornersToPass > 0; numCornersToPass-- {
		for iter := 0; iter < toIter/2 && len(inputSteps) > 3; iter++ {
			select {
			case <-ctx.Done():
				return inputSteps
			default:
			}
			// get start node of first edge. Cannot be either the last or second-to-last node.
			// Intn will return an int in the half-open interval [0,n)
			i := mp.randseed.Intn(len(inputSteps) - 2)
			j := i + 1
			cornersPassed := 0
			hitCorners := []node{}
			for (cornersPassed != numCornersToPass || !inputSteps[j].Corner()) && j < len(inputSteps)-1 {
				j++
				if cornersPassed < numCornersToPass && inputSteps[j].Corner() {
					cornersPassed++
					hitCorners = append(hitCorners, inputSteps[j])
				}
			}
			// no corners existed between i and end of inputSteps -> not good candidate for smoothing
			if len(hitCorners) == 0 {
				continue
			}

			shortcutGoal := make(map[node]node)

			iSol := inputSteps[i]
			jSol := inputSteps[j]
			shortcutGoal[jSol] = nil

			mp.constrainedExtend(ctx, mp.randseed, shortcutGoal, jSol, iSol, schan)
			reached := <-schan

			// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
			// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
			// so we allow elongation here.
			dist := mp.planOpts.DistanceFunc(&ik.Segment{StartConfiguration: inputSteps[i].Q(), EndConfiguration: reached.Q()})
			if dist < mp.planOpts.JointSolveDist {
				for _, hitCorner := range hitCorners {
					hitCorner.SetCorner(false)
				}

				newInputSteps := append([]node{}, inputSteps[:i]...)
				for reached != nil {
					newInputSteps = append(newInputSteps, reached)
					reached = shortcutGoal[reached]
				}
				newInputSteps[i].SetCorner(true)
				newInputSteps[len(newInputSteps)-1].SetCorner(true)
				newInputSteps = append(newInputSteps, inputSteps[j+1:]...)
				inputSteps = newInputSteps
			}
		}
	}
	return inputSteps
}

// getFrameSteps will return a slice of positive values representing the largest amount a particular DOF of a frame should
// move in any given step.
func getFrameSteps(f referenceframe.Frame, by float64) []float64 {
	dof := f.DoF()
	pos := make([]float64, len(dof))
	for i, lim := range dof {
		l, u := lim.Min, lim.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		jRange := math.Abs(u - l)
		pos[i] = jRange * by
	}
	return pos
}
