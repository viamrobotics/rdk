package armplanning

import (
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

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

	// TODO: this is duplicated work as it's also done in motionplan.NewConstraintHandler
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
	if err != nil {
		return nil, err
	}

	movingRobotGeometries, staticRobotGeometries := motionChains.geometries(fs, frameSystemGeometries)

	return motionplan.NewConstraintHandler(
		opt.CollisionBufferMM,
		logger,
		constraints,
		startPoses, goalPoses,
		fs,
		movingRobotGeometries, staticRobotGeometries,
		seedMap,
		worldState,
		boundingRegions,
		false,
	)
}
