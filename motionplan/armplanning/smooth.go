package armplanning

import (
	"context"
	// "math".
	"time"

	// "go.viam.com/rdk/motionplan".
	"go.viam.com/rdk/referenceframe"
)

func simpleSmoothStep(psc *planSegmentContext, steps []referenceframe.FrameSystemInputs, step int) []referenceframe.FrameSystemInputs {
	// look at each triplet, see if we can remove the middle one
	for i := step + 1; i < len(steps); i += step {
		err := psc.checkPath(steps[i-step-1], steps[i])
		if err != nil {
			continue
		}
		// we can merge
		steps = append(steps[0:i-step], steps[i:]...)
		i--
	}
	return steps
}

// smoothPath will pick two points at random along the path and attempt to do a fast gradient descent directly between
// them, which will cut off randomly-chosen points with odd joint angles into something that is a more intuitive motion.
func smoothPathSimple(ctx context.Context, psc *planSegmentContext,
	steps []referenceframe.FrameSystemInputs,
) []referenceframe.FrameSystemInputs {
	start := time.Now()

	originalSize := len(steps)
	steps = simpleSmoothStep(psc, steps, 10)
	steps = simpleSmoothStep(psc, steps, 1)

	if len(steps) != originalSize {
		psc.pc.logger.Debugf("simpleSmooth %d -> %d in %v", originalSize, len(steps), time.Since(start))
		return smoothPath(ctx, psc, steps)
	}
	return steps
}

func smoothPath(ctx context.Context, psc *planSegmentContext, steps []referenceframe.FrameSystemInputs) []referenceframe.FrameSystemInputs {
	steps = smoothPathSimple(ctx, psc, steps)
	/*
		toIter := int(math.Min(float64(len(steps)*len(steps)), float64(psc.pc.planOpts.SmoothIter)))

		corners := make([]bool, len(steps))

		for numCornersToPass := 2; numCornersToPass > 0; numCornersToPass-- {
			for iter := 0; iter < toIter/2 && len(steps) > 3; iter++ {
				select {
				case <-ctx.Done():
					return steps
				default:
				}
				// get start node of first edge. Cannot be either the last or second-to-last node.
				// Intn will return an int in the half-open interval [0,n)
				i := psc.pc.randseed.Intn(len(steps) - 2)
				j := i + 1
				cornersPassed := 0
				hitCorners := []*node{}
				for (cornersPassed != numCornersToPass || !steps[j].corner) && j < len(steps)-1 {
					j++
					if cornersPassed < numCornersToPass && steps[j].corner {
						cornersPassed++
						hitCorners = append(hitCorners, steps[j])
					}
				}
				// no corners existed between i and end of steps -> not good candidate for smoothing
				if len(hitCorners) == 0 {
					continue
				}

				shortcutGoal := make(rrtMap)

				iSol := steps[i]
				jSol := steps[j]
				shortcutGoal[jSol] = nil

				reached := mp.constrainedExtend(ctx, i, shortcutGoal, jSol, iSol)

				// Note this could technically replace paths with "longer" paths i.e. with more waypoints.
				// However, smoothed paths are invariably more intuitive and smooth, and lend themselves to future shortening,
				// so we allow elongation here.
				dist := mp.configurationDistanceFunc(&motionplan.SegmentFS{
					StartConfiguration: steps[i].inputs,
					EndConfiguration:   reached.inputs,
				})
				if dist < mp.planOpts.InputIdentDist {
					for _, hitCorner := range hitCorners {
						hitCorner.corner = false
					}

					newInputSteps := append([]*node{}, steps[:i]...)
					for reached != nil {
						newInputSteps = append(newInputSteps, reached)
						reached = shortcutGoal[reached]
					}
					newInputSteps[i].corner = true
					newInputSteps[len(newInputSteps)-1].corner = true
					newInputSteps = append(newInputSteps, steps[j+1:]...)
					steps = newInputSteps
				}
			}
		}
	*/
	return steps
}
