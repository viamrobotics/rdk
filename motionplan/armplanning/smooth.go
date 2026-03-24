package armplanning

import (
	"context"
	"slices"
	"time"

	"go.viam.com/utils/trace"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

func simpleSmoothStep(ctx context.Context, psc *planSegmentContext, steps []*referenceframe.LinearInputs, step int,
) []*referenceframe.LinearInputs {
	ctx, span := trace.StartSpan(ctx, "simpleSmoothStep")
	defer span.End()
	// look at each triplet, see if we can remove the middle one
	for i := step + 1; i < len(steps); i += step {
		err := psc.checkPath(ctx, steps[i-step-1], steps[i], false)
		if err != nil {
			continue
		}
		// we can merge
		steps = append(steps[0:i-step], steps[i:]...)
		i -= step
	}
	return steps
}

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func smoothPathSimple(ctx context.Context, psc *planSegmentContext,
	steps []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	ctx, span := trace.StartSpan(ctx, "smoothPathSimple")
	defer span.End()
	start := time.Now()

	originalSize := len(steps)
	steps = simpleSmoothStep(ctx, psc, steps, 10)
	steps = simpleSmoothStep(ctx, psc, steps, 3)
	steps = simpleSmoothStep(ctx, psc, steps, 1)

	steps = tryOnlyMovingComponentsThatNeedToMove(ctx, psc, steps)

	if len(steps) != originalSize {
		psc.pc.logger.Debugf("simpleSmooth %d -> %d in %v", originalSize, len(steps), time.Since(start))
	}
	return steps
}

func smoothPath(
	ctx context.Context, psc *planSegmentContext, steps []*referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "smoothPlan")
	defer span.End()
	var err error
	steps = smoothPathSimple(ctx, psc, steps)
	if !psc.pc.request.myTestOptions.doNotCloseObstacles {
		steps, err = addCloseObstacleWaypoints(ctx, psc, steps)
		if err != nil {
			return nil, err
		}
	}
	return steps, nil
}

// addCloseObstacleWaypoints interpolates between waypoints and adds new waypoints
// where the path comes within twice the minimum distance of an obstacle.
// This prevents the smoothed path from getting too close to obstacles during interpolation.
func addCloseObstacleWaypoints(
	ctx context.Context, psc *planSegmentContext, steps []*referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, error) {
	ctx, span := trace.StartSpan(ctx, "addCloseObstacleWaypoints")
	defer span.End()

	if len(steps) < 2 {
		return steps, nil
	}

	result := []*referenceframe.LinearInputs{steps[0]}

	for i := 1; i < len(steps); i++ {
		// Get waypoints that are close to obstacles in this segment
		closeWaypoints, err := findCloseObstacleWaypoints(ctx, psc, steps[i-1], steps[i])
		if err != nil {
			return nil, err
		}

		// Add close waypoints before the current step
		result = append(result, closeWaypoints...)
		result = append(result, steps[i])
	}

	if len(result) != len(steps) {
		psc.pc.logger.Debugf("addCloseObstacleWaypoints: added %d waypoints (%d -> %d)",
			len(result)-len(steps), len(steps), len(result))
	}

	return result, nil
}

// findCloseObstacleWaypoints interpolates between start and end configurations
// and returns configurations where the robot has a local minimum distance to obstacles
// less than twice the global min distance. Instead of adding every point within the threshold,
// this finds contiguous "close zones" and adds only the point of closest approach
// in each zone.
func findCloseObstacleWaypoints(
	ctx context.Context,
	psc *planSegmentContext,
	start, end *referenceframe.LinearInputs,
) ([]*referenceframe.LinearInputs, error) {
	segment := &motionplan.SegmentFS{
		StartConfiguration: start,
		EndConfiguration:   end,
		FS:                 psc.pc.fs,
	}

	interpolated, err := motionplan.InterpolateSegmentFS(segment, psc.pc.planOpts.Resolution)
	if err != nil {
		return nil, err
	}

	if len(interpolated) < 3 {
		return nil, nil
	}

	var closeWaypoints []*referenceframe.LinearInputs

	for i := 1; i < len(interpolated)-1; i++ {
		state := &motionplan.StateFS{
			FS:            psc.pc.fs,
			Configuration: interpolated[i],
		}

		closestObstacle, err := psc.checker.CheckStateFSConstraints(ctx, state)
		if err != nil {
			return nil, err
		}

		if closestObstacle < max(.1, 10*psc.pc.planOpts.CollisionBufferMM) {
			closeWaypoints = append(closeWaypoints, interpolated[i])
		}
	}

	return closeWaypoints, nil
}

func tryOnlyMovingComponentsThatNeedToMove(ctx context.Context, psc *planSegmentContext,
	steps []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	moving, _ := psc.motionChains.framesFilteredByMovingAndNonmoving()

	for idx := 1; idx < len(steps); idx++ {
		curr := steps[idx]
		prev := steps[idx-1]

		updated := curr.Copy()

		for component, currInputs := range curr.Items() {
			if slices.Contains(moving, component) {
				continue
			}

			if len(currInputs) == 0 {
				continue
			}

			prevInputs := prev.Get(component)

			updated.Put(component, prevInputs)
		}

		err := psc.checkPath(ctx, prev, updated, false)
		if err == nil {
			steps[idx] = updated
		}
	}

	return steps
}
