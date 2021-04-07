package kinematics

import (
	"go.viam.com/robotcore/kinematics/kinmath"
)

type Goal struct {
	GoalTransform *kinmath.QuatTrans
	EffectorID    int
}

type InverseKinematics interface {
	AddGoal(*kinmath.QuatTrans, int)
	ClearGoals()
	GetGoals() []Goal
	Solve() bool
	SetID(int)
	GetID() int
	GetMdl() *Model
	Halt()
}

// Returns the dot product of a vector with itself
func SquaredNorm(vec []float64) float64 {
	norm := 0.0
	for _, v := range vec {
		norm += v * v
	}
	return norm
}
