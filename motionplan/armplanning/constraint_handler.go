package armplanning

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var defaultMinStepCount = 2

func newConstraintHandler(
	opt *PlannerOptions,
	logger logging.Logger,
	constraints *motionplan.Constraints,
	from, to *PlanState,
	fs *referenceframe.FrameSystem,
	motionChains *motionChains,
	seedMap referenceframe.FrameSystemInputs,
	worldState *referenceframe.WorldState,
	boundingRegions []spatialmath.Geometry,
) (*motionplan.ConstraintHandler, error) {
	startPoses, err := from.ComputePoses(fs)
	if err != nil {
		return nil, err
	}
	goalPoses, err := to.ComputePoses(fs)
	if err != nil {
		return nil, err
	}

	movingRobotGeometries, staticRobotGeometries := motionChains.geometries(fs, frameSystemGeometries)

	return motionChains.NewConstraintHandler(
		opt.CollisionBufferMM,
		logger,
		constraints,
		startPoses, goalPoses,
		fs,
		movingRobotGeometries, staticRobotGeometries,
		seedMap,
		worldState,
		boundingRegions,
	)
}
