package armplanning

import (
	"context"
	// "math".
	"time"

	// "go.viam.com/rdk/motionplan".
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/referenceframe"
)

func simpleSmoothStep(ctx context.Context, psc *planSegmentContext, steps []*referenceframe.LinearInputs, step int,
) []*referenceframe.LinearInputs {
	ctx, span := trace.StartSpan(ctx, "simpleSmoothStep")
	defer span.End()
	// look at each triplet, see if we can remove the middle one
	for i := step + 1; i < len(steps); i += step {
		err := psc.checkPath(ctx, steps[i-step-1], steps[i])
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

	if len(steps) != originalSize {
		psc.pc.logger.Debugf("simpleSmooth %d -> %d in %v", originalSize, len(steps), time.Since(start))
	}
	return steps
}

func smoothPath(
	ctx context.Context, psc *planSegmentContext, steps []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	ctx, span := trace.StartSpan(ctx, "smoothPlan")
	defer span.End()
	steps = smoothPathSimple(ctx, psc, steps)
	return steps
}
