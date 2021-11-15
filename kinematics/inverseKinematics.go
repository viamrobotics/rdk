package kinematics

import (
	"context"

	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
)

// Motions with swing values less than this are considered good enough to do without looking for better ones
const (
	goodSwingAmt = 1.6
	waypoints    = 4
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
	Solve(context.Context, chan []frame.Input, spatial.Pose, []frame.Input) error
	SetSolveWeights(frame.SolverDistanceWeights)
	SetGradient(func(spatial.Pose, spatial.Pose) float64)
	Close() error
}

// toArray returns the frame.SolverDistanceWeights as a slice with the components in the same order as the array returned from
// pose ToDelta. Note that orientation components are multiplied by 100 since they are usually small to avoid drift.
func toArray(dc frame.SolverDistanceWeights) []float64 {
	return []float64{dc.Trans.X, dc.Trans.Y, dc.Trans.Z, 100 * dc.Orient.TH * dc.Orient.X, 100 * dc.Orient.TH * dc.Orient.Y, 100 * dc.Orient.TH * dc.Orient.Z}
}

// SquaredNorm returns the dot product of a vector with itself
func SquaredNorm(vec []float64) float64 {
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	return norm
}

// WeightedSquaredNorm returns the dot product of a vector with itself, applying the given weights to each piece.
func WeightedSquaredNorm(vec []float64, config frame.SolverDistanceWeights) float64 {
	configArr := toArray(config)
	norm := 0.0
	for i, v := range vec {
		norm += v * v * configArr[i]
	}
	return norm
}

func limitsToArrays(limits []frame.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}
