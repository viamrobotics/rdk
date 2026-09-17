package armplanning

import (
	"context"
	"fmt"
	"slices"
	"time"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

func simpleSmoothStep(ctx context.Context, psc *PlanSegmentContext, steps []*referenceframe.LinearInputs, step int,
	budget *int,
) []*referenceframe.LinearInputs {
	// look at each triplet, see if we can remove the middle one
	for i := step + 1; i < len(steps); i += step {
		if *budget <= 0 {
			return steps
		}
		*budget--
		err := psc.CheckPath(ctx, steps[i-step-1], steps[i], false, nil)
		if err != nil {
			continue
		}
		// we can merge
		steps = append(steps[0:i-step], steps[i:]...)
		i -= step
	}
	return steps
}

func smoothMultiArms(
	ctx context.Context, psc *PlanSegmentContext, steps []*referenceframe.LinearInputs,
) {
	if len(psc.pc.movableFrames) == 1 {
		return
	}

	start := 0
	end := len(steps) - 1

nextFrame:
	for _, frameToModify := range psc.pc.movableFrames {
		// For each frame run an experiment. We'll change the steps for the current `frameToModify`
		// while leaving the rest as they are. We'll change each `frameToModify` to do a
		// straight-line interpolation between the start and end step. To smooth out unnecessary 0
		// -> 2 -> 1 movements. Or more critically, 0 -> 1 -> 0.
		//
		// These can arise when falling down to cbirrt to solve for one arm while a different arm
		// didn't need cbirrt.
		frameToModifyStartInps := steps[start].Get(frameToModify)
		frameToModifyEndInps := steps[end].Get(frameToModify)
		stepSize := make([]referenceframe.Input, len(frameToModifyStartInps))
		for frameDoFIdx := range stepSize {
			diff := (frameToModifyEndInps[frameDoFIdx] - frameToModifyStartInps[frameDoFIdx])
			numSteps := float64(end - start)
			stepSize[frameDoFIdx] = diff / numSteps
		}

		// `prev` keeps track of the "start" of the arm for each movement element in the `steps` trajectory.
		prev := steps[start]

		// `acc`umulate new candidates into a container. If the whole candidate path succeeds, this
		// is used to craft the new trajectory.
		acc := make([][]referenceframe.Input, 0, end-start-1)
		for timeIdx := 1; timeIdx < len(steps); timeIdx++ {
			tmpStep := steps[timeIdx].Copy()
			for frameName := range tmpStep.Keys() {
				if frameName != frameToModify {
					continue
				}

				// `inpsAtStep` is a window over the underlying linear inputs. Assigning to this
				// will modify our copy in place.
				inpsAtStep := tmpStep.Get(frameName)
				for jointIdx, startVal := range frameToModifyStartInps {
					// The position of `jointIdx` at time `timeIdx` is the initial position plus the
					// number of steps (time units) we've taken.
					inpsAtStep[jointIdx] = startVal + (float64(timeIdx) * stepSize[jointIdx])
				}
				acc = append(acc, inpsAtStep)
			}

			err := psc.CheckPath(ctx, prev, tmpStep, true, nil)
			if err != nil {
				// Doing a straight-line interpolation for `frameToModify` failed. Leave this arm's
				// path alone and try the next one.
				continue nextFrame
			}

			prev = tmpStep
		}

		// We succeeded in straight-line interpolating `frameToModify`. Walk over the `acc`umulated
		// modifications and write them back into the input `steps`.
		for accIdx, accStep := range acc {
			stepTargetIdx := start + accIdx + 1
			steps[stepTargetIdx].Put(frameToModify, accStep)
		}
	}
}

// smoothProbeBudget caps the total number of coarse shortcut probes one
// smoothing invocation may spend. Smoothing has diminishing returns - the
// close-obstacle sweep afterwards validates whatever shape remains - and an
// uncapped pass over a long raw search path could cost more than the search.
const smoothProbeBudget = 300

// vertexPullStep shrinks detours that waypoint removal cannot: when deleting
// an interior waypoint is blocked (the straight bypass clips an obstacle),
// the waypoint can often still be pulled partway toward the midpoint of its
// neighbors, tightening the arc around the obstacle. Without this, a path
// routed through an arbitrary intermediate configuration (a roadmap sample, a
// search tree node) keeps that configuration's full detour forever - which
// also means harvesting locks the detour into every future replan. Every
// accepted pull strictly lowers path cost, so a few rounds converge.
func vertexPullStep(ctx context.Context, psc *PlanSegmentContext, steps []*referenceframe.LinearInputs,
	budget *int,
) []*referenceframe.LinearInputs {
	if len(steps) < 3 {
		return steps
	}
	segCost := func(a, b *referenceframe.LinearInputs) float64 {
		return psc.pc.ConfigurationDistanceFunc(&motionplan.SegmentFS{StartConfiguration: a, EndConfiguration: b})
	}
	// Only paths carrying substantial detour are worth the CheckPath spend:
	// a path near the direct start-goal cost has nothing to reclaim (and a
	// converged, previously-pulled path re-entering smoothing skips straight
	// through here on replans).
	const vertexPullSlackFactor = 1.5
	total := 0.0
	for i := 1; i < len(steps); i++ {
		total += segCost(steps[i-1], steps[i])
	}
	direct := segCost(steps[0], steps[len(steps)-1])
	if total <= vertexPullSlackFactor*direct {
		return steps
	}
	const pullRounds = 6
	for r := 0; r < pullRounds; r++ {
		improved := false
		for i := 1; i+1 < len(steps); i++ {
			prev, cur, next := steps[i-1], steps[i], steps[i+1]
			mid, err := referenceframe.InterpolateFS(psc.pc.fs, prev, next, 0.5)
			if err != nil {
				return steps
			}
			curCost := segCost(prev, cur) + segCost(cur, next)
			// Blend from cur toward the neighbors' midpoint, most aggressive
			// first. The full midpoint itself is never a candidate: it lies on
			// the straight prev-next line, which waypoint removal already
			// proved blocked whenever this step matters.
			for _, alpha := range []float64{0.75, 0.5, 0.25} {
				cand, err := referenceframe.InterpolateFS(psc.pc.fs, cur, mid, alpha)
				if err != nil {
					return steps
				}
				if segCost(prev, cand)+segCost(cand, next) >= curCost-1e-9 {
					continue
				}
				if *budget <= 0 {
					return steps
				}
				*budget -= 2
				if psc.CheckPath(ctx, prev, cand, false, nil) != nil || psc.CheckPath(ctx, cand, next, false, nil) != nil {
					continue
				}
				steps[i] = cand
				improved = true
				break
			}
		}
		if !improved {
			break
		}
	}
	return steps
}

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func smoothPathSimple(ctx context.Context, psc *PlanSegmentContext,
	steps []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	ctx, span := trace.StartSpan(ctx, "smoothPathSimple")
	defer span.End()
	start := time.Now()

	originalSize := len(steps)
	budget := smoothProbeBudget
	if len(steps) > 40 {
		steps = simpleSmoothStep(ctx, psc, steps, 10, &budget)
	}
	if len(steps) > 15 {
		steps = simpleSmoothStep(ctx, psc, steps, 3, &budget)
	}
	steps = simpleSmoothStep(ctx, psc, steps, 1, &budget)
	steps = vertexPullStep(ctx, psc, steps, &budget)
	// Pulled vertices can straighten segments enough to make whole waypoints
	// removable, so run one more removal pass.
	steps = simpleSmoothStep(ctx, psc, steps, 1, &budget)

	steps = tryOnlyMovingComponentsThatNeedToMove(ctx, psc, steps)

	if len(steps) != originalSize {
		psc.pc.logger.Debugf("simpleSmooth %d -> %d in %v", originalSize, len(steps), time.Since(start))
	}
	return steps
}

// smoothPath returns the final trajectory and, separately, the compact
// smoothed path from before close-obstacle waypoint insertion. Harvesting
// must use the compact form: the close waypoints are interpolation guards,
// not corridor structure, and feeding them back into the roadmap makes every
// replan's path longer than the last.
func smoothPath(
	ctx context.Context, psc *PlanSegmentContext, steps []*referenceframe.LinearInputs,
) (final, compact []*referenceframe.LinearInputs, err error) {
	ctx, span := trace.StartSpan(ctx, "smoothPlan")
	defer span.End()

	compact = smoothPathSimple(ctx, psc, steps)
	// The above smoothing does smaller straight-line interpolations for all inputs at once. In the
	// case of multiple arms we can sometimes smooth one arm but not the other. This follow-up call
	// handles a narrow branch of egregious cases.
	//
	// `compact` is modified in place.
	smoothMultiArms(ctx, psc, compact)

	if psc.pc.request.myTestOptions.doNotCloseObstacles {
		return compact, compact, nil
	}
	final, err = addCloseObstacleWaypoints(ctx, psc, compact)
	return final, compact, err
}

// addCloseObstacleWaypoints interpolates every segment at full resolution and
// inserts waypoints wherever the path comes close to an obstacle, preventing
// later interpolation from cutting corners.
func addCloseObstacleWaypoints(
	ctx context.Context, psc *PlanSegmentContext, steps []*referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "addCloseObstacleWaypoints")
	defer span.End()

	if len(steps) < 2 {
		return steps, nil
	}

	result := []*referenceframe.LinearInputs{steps[0]}
	for i := 1; i < len(steps); i++ {
		closeWaypoints, blocked, err := findCloseObstacleWaypoints(ctx, psc, steps[i-1], steps[i])
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, fmt.Errorf("smoothed path segment %d invalid at full resolution", i)
		}
		result = append(result, closeWaypoints...)
		result = append(result, steps[i])
	}

	if len(result) != len(steps) {
		psc.pc.logger.Debugf("addCloseObstacleWaypoints: added %d waypoints (%d -> %d)",
			len(result)-len(steps), len(steps), len(result))
	}
	return result, nil
}

// findCloseObstacleWaypoints interpolates between start and end at full
// resolution, verifying every state, and reports configurations that come
// within the close-obstacle threshold. blocked is true when any state violates
// a constraint. States whose clearance proves the next several interpolation
// steps cannot collide are skipped for collision purposes (their cheap
// topological constraints are still verified individually).
func findCloseObstacleWaypoints(
	ctx context.Context,
	psc *PlanSegmentContext,
	start, end *referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, bool, error) {
	segment := &motionplan.SegmentFS{
		StartConfiguration: start,
		EndConfiguration:   end,
		FS:                 psc.pc.fs,
	}

	resolution := psc.pc.planOpts.Resolution
	interpolated, err := motionplan.InterpolateSegmentFS(segment, resolution)
	if err != nil {
		return nil, false, err
	}

	if len(interpolated) < 3 {
		return nil, false, nil
	}

	closeThreshold := max(.1, 10*psc.pc.planOpts.CollisionBufferMM)

	var closeWaypoints []*referenceframe.LinearInputs

	for i := 1; i < len(interpolated)-1; i++ {
		state := &motionplan.StateFS{
			FS:            psc.pc.fs,
			Configuration: interpolated[i],
		}

		closestObstacle, err := psc.Checker.CheckStateFSConstraints(ctx, state)
		if err != nil {
			return nil, true, nil //nolint:nilerr // constraint violation is the blocked verdict, not an error
		}

		if closestObstacle < closeThreshold {
			closeWaypoints = append(closeWaypoints, interpolated[i])
			continue
		}

		// Clearance-bound skipping (same argument as the segment checker's):
		// nothing can collide within the next canSkip steps, and no state in
		// them can be within the close threshold either. Topological
		// constraints carry no such bound but are cheap; verify them per
		// skipped state.
		canSkip := int(min(100, (closestObstacle-closeThreshold)/resolution))
		for j := i + 1; j <= i+canSkip && j < len(interpolated)-1; j++ {
			if err := psc.Checker.CheckStateFSTopoOnly(&motionplan.StateFS{
				FS:            psc.pc.fs,
				Configuration: interpolated[j],
			}); err != nil {
				return nil, true, nil //nolint:nilerr // constraint violation is the blocked verdict, not an error
			}
		}
		i += canSkip
	}

	return closeWaypoints, false, nil
}

func tryOnlyMovingComponentsThatNeedToMove(ctx context.Context, psc *PlanSegmentContext,
	steps []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	moving, _ := psc.motionChains.framesFilteredByMovingAndNonmoving()

	for idx := 1; idx < len(steps); idx++ {
		curr := steps[idx]
		prev := steps[idx-1]

		updated := curr.Copy()

		changed := false
		for component, currInputs := range curr.Items() {
			if slices.Contains(moving, component) {
				continue
			}

			if len(currInputs) == 0 {
				continue
			}

			prevInputs := prev.Get(component)
			if referenceframe.InputsL2Distance(prevInputs, currInputs) == 0 {
				continue
			}

			updated.Put(component, prevInputs)
			changed = true
		}
		// Nothing to pin down (the non-moving components already hold still):
		// skip the full path check entirely.
		if !changed {
			continue
		}

		err := psc.CheckPath(ctx, prev, updated, false, nil)
		if err == nil {
			steps[idx] = updated
		}
	}

	return steps
}
