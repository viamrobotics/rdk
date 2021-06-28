package kinematics

import (
	"go.viam.com/core/spatialmath"
)

// Goal TODO
type Goal struct {
	GoalTransform *spatialmath.DualQuaternion
	EffectorID    int
}

// InverseKinematics TODO
type InverseKinematics interface {
	AddGoal(*spatialmath.DualQuaternion, int)
	ClearGoals()
	GetGoals() []Goal
	Solve() bool
	SetID(int)
	GetID() int
	GetMdl() *Model
	Halt()
}

// toArray returns the DistanceConfig as a slice with the components in the same order as the array returned from ToDelta
func (dc *DistanceConfig) toArray() []float64 {
	return []float64{dc.Trans.X, dc.Trans.Y, dc.Trans.Z, dc.Orient.TH, dc.Orient.X, dc.Orient.Y, dc.Orient.Z}
}

// SquaredNorm returns the dot product of a vector with itself
func SquaredNorm(vec []float64) float64 {
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	return norm
}

// WeightedSquaredNorm TODO
func WeightedSquaredNorm(vec []float64, config DistanceConfig) float64 {
	configArr := config.toArray()
	norm := 0.0
	for i, v := range vec {
		norm += v * v * configArr[i]
	}
	// With R4 angle axis, we wind up at 0 1 0 0
	return norm - 1
}
