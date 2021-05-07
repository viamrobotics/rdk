package kinematics

import (
	"math"
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/robotcore/kinematics/kinmath"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m, logger)

	pos := pb.ArmPosition{X: 170, Y: 0, Z: 180, RX: 0, RY: 0, RZ: 0}
	transform := kinmath.NewQuatTransFromRotation(pos.RX, pos.RY, pos.RZ)
	transform.SetX(float64(pos.X))
	transform.SetY(float64(pos.Y))
	transform.SetZ(float64(pos.Z))

	ik.AddGoal(transform, 0)

	m.SetPosition([]float64{1, 1, 1, 1, 1, 0})

	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)

	pos = pb.ArmPosition{X: -46, Y: -133, Z: 372, RX: -18, RY: -33, RZ: -11}
	transform = kinmath.NewQuatTransFromRotation(pos.RX*math.Pi/180, pos.RY*math.Pi/180, pos.RZ*math.Pi/180)
	transform.SetX(float64(pos.X / 2))
	transform.SetY(float64(pos.Y / 2))
	transform.SetZ(float64(pos.Z / 2))

	ik.AddGoal(transform, 0)

	newPos := []float64{49, 28, -101, 0, -73, 0}
	for i := range newPos {
		newPos[i] *= math.Pi / 180
	}
	m.SetPosition(newPos)

	solved = ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}
