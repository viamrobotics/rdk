package kinematics

import (
	"math"
	"testing"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.viam.com/robotcore/kinematics/kinmath"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m, logger)

	pos := Position{170, 0, 180, 0, 0, 0}
	transform := kinmath.NewQuatTransFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X)
	transform.SetY(pos.Y)
	transform.SetZ(pos.Z)

	ik.AddGoal(transform, 0)

	m.SetPosition([]float64{1, 1, 1, 1, 1, 0})

	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)

	pos = Position{-46.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	transform = kinmath.NewQuatTransFromRotation(pos.Rx*math.Pi/180, pos.Ry*math.Pi/180, pos.Rz*math.Pi/180)
	transform.SetX(pos.X / 2)
	transform.SetY(pos.Y / 2)
	transform.SetZ(pos.Z / 2)

	ik.AddGoal(transform, 0)

	newPos := []float64{49.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379}
	for i := range newPos {
		newPos[i] *= math.Pi / 180
	}
	m.SetPosition(newPos)

	solved = ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}
