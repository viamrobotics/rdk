package motionplan

import (
	"context"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// MotionPlanner is maybe a future interface for internal go motion planners.
type MotionPlanner interface {
	PlanFrameMotion(ctx context.Context,
		logger logging.Logger,
		dst spatialmath.Pose,
		f referenceframe.Frame,
		seed []referenceframe.Input,
		constraints *Constraints,
		planningOpts map[string]interface{},
	) ([][]referenceframe.Input, error)
}

var global MotionPlanner

// GetGlobal - get the global motion planner or panic.
func GetGlobal() MotionPlanner {
	if global == nil {
		panic("no global MotionPlanner")
	}
	return global
}

// RegisterGlobal - register the global motion planner, panic if there is already one.
func RegisterGlobal(mp MotionPlanner) {
	if global != nil {
		panic("already have a global MotionPlanner")
	}
	global = mp
}
