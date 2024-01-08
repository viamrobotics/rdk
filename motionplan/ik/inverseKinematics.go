// Package ik contains tols for doing gradient-descent based inverse kinematics, allowing for the minimization of arbitrary metrics
// based on the output of calling `Transform` on the given frame.
package ik

import (
	"context"

	"go.viam.com/rdk/referenceframe"
)

const (
	// Default distance below which two distances are considered equal.
	defaultEpsilon = 0.001

	// default amount of closeness to get to the goal.
	defaultGoalThreshold = defaultEpsilon * defaultEpsilon
)

// InverseKinematics defines an interface which, provided with seed inputs and a Metric to minimize to zero, will output all found
// solutions to the provided channel until cancelled or otherwise completes.
type InverseKinematics interface {
	referenceframe.Limited
	// Solve receives a context, the goal arm position, and current joint angles.
	Solve(context.Context, chan<- *Solution, []referenceframe.Input, StateMetric, int) error
}

// Solution is the struct returned from an IK solver. It contains the solution configuration, the score of the solution, and a flag
// indicating whether that configuration and score met the solution criteria requested by the caller.
type Solution struct {
	Configuration []referenceframe.Input
	Score         float64
	Exact         bool
}
