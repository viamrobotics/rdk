package motionplan

import (
	"context"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

// InverseKinematics defines an interface which, provided with a goal position and seed inputs, will output all found
// solutions to the provided channel until cancelled or otherwise completes.
type InverseKinematics interface {
	// Solve receives a context, the goal arm position, and current joint angles.
	Solve(context.Context, chan<- []referenceframe.Input, spatial.Pose, []referenceframe.Input, Metric, int) error
}

func limitsToArrays(limits []referenceframe.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}
