package armplanning

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type myMotionPlanner struct{}

func (mmp *myMotionPlanner) PlanFrameMotion(ctx context.Context,
	logger logging.Logger,
	dst spatialmath.Pose,
	f referenceframe.Frame,
	seed []referenceframe.Input,
	constraints *motionplan.Constraints,
	planningOpts map[string]interface{},
) ([][]referenceframe.Input, error) {
	return errtrace.Wrap2(PlanFrameMotion(ctx, logger, dst, f, seed, constraints, planningOpts))
}

func init() {
	motionplan.RegisterGlobal(&myMotionPlanner{})
}
