package kinematics

import (
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/kinematics/kinmath"
	"go.viam.com/robotcore/testutils"
)

type Position struct {
	X, Y, Z float64 // meters of the end effector from the base

	Rx, Ry, Rz float64 // angular orientation about each axis, in degrees
}

func TestCreateIKSolver(t *testing.T) {
	m, err := ParseJSONFile(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m)

	pos := Position{0, -365, 360.25, 0, 0, 0}
	transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	ik.AddGoal(transform, 0)

	m.SetPosition([]float64{1, 1, 1, 0, 0, 1})

	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}
