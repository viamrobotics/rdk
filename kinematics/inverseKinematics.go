package kinematics

import (
	"context"

	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// goal contains a pose representing a location and orientation to try to reach, and the ID of the end
// effector which should be trying to reach it.
type goal struct {
	GoalTransform spatial.Pose
	EffectorID    int
}

// InverseKinematics defines an interface which, provided with a goal position and seed inputs, will output all found
// solutions to the provided channel until cancelled or otherwise completes
type InverseKinematics interface {
	// Solve receives a context, the goal arm position, and current joint angles.
	Solve(ctx context.Context, c chan<- []frame.Input, goal spatial.Pose, seed []frame.Input) error
	SetMetric(Metric)
	Close() error
}

func limitsToArrays(limits []frame.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}
