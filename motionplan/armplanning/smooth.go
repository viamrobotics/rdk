package armplanning

import (
	"context"
	"time"

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
func smoothPath(ctx context.Context, psc *planSegmentContext, steps []referenceframe.FrameSystemInputs) []referenceframe.FrameSystemInputs {
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
