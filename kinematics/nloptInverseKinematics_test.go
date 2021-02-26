package kinematics

import (
	"testing"
)

func TestCreateIKSolver(t *testing.T) {
	ik := CreateIKSolver(&Model{})
	ik.Solve()
}
