package kinematics

import (
	"context"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
)

// goal contains a dual quaternion representing a location and orientation to try to reach, and the ID of the end
// effector which should be trying to reach it.
type goal struct {
	GoalTransform *spatialmath.DualQuaternion
	EffectorID    int
}

// InverseKinematics TODO
type InverseKinematics interface {
	// Solve receives a context, the goal arm position, and current joint angles.
	// It will return a boolean which will be true if it solved successfully, and the joint positions which
	// will yield that goal position.
	Solve(context.Context, *pb.ArmPosition, *pb.JointPositions) (*pb.JointPositions, error)
	Mdl() *Model
}

// toArray returns the SolverDistanceWeights as a slice with the components in the same order as the array returned from ToDelta
func (dc *SolverDistanceWeights) toArray() []float64 {
	return []float64{dc.Trans.X, dc.Trans.Y, dc.Trans.Z, dc.Orient.TH, dc.Orient.X, dc.Orient.Y, dc.Orient.Z}
}

// SquaredNorm returns the dot product of a vector with itself
func SquaredNorm(vec []float64) float64 {
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	return norm - 1
}

// WeightedSquaredNorm TODO
func WeightedSquaredNorm(vec []float64, config SolverDistanceWeights) float64 {
	configArr := config.toArray()
	norm := 0.0
	for i, v := range vec {
		norm += v * v * configArr[i]
	}
	// With R4 angle axis, we wind up at 0 1 0 0
	return norm - 1
}
