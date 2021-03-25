package kinematics

import (
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.viam.com/robotcore/kinematics/kinmath"
)

type Position struct {
	X, Y, Z float64 // meters of the end effector from the base

	Rx, Ry, Rz float64 // angular orientation about each axis, in degrees
}

func TestCreateIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m, logger)

	pos := Position{90, -165, 360.25, 0, 45, 45}
	transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	ik.AddGoal(transform, 0)

	m.SetPosition([]float64{1, 1, 1, 0, 0, 1})

	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}
