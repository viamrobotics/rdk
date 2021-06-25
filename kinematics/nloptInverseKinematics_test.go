package kinematics

import (
	"math"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/kinematics/kinmath"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m, logger)

	pos := pb.ArmPosition{X: 360, Z: 362}
	transform := kinmath.NewQuatTransFromArmPos(&pos)

	ik.AddGoal(transform, 0)

	m.SetPosition([]float64{1, 1, 1, 1, 1, 0})
	m.ForwardPosition()

	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)

	pos = pb.ArmPosition{X: -46, Y: -23, Z: 372, Theta: utils.RadToDeg(3.92), OX: -0.46, OY: 0.84, OZ: 0.28}
	transform = kinmath.NewQuatTransFromArmPos(&pos)

	ik.AddGoal(transform, 0)

	newPos := []float64{49, 28, -101, 0, -73, 0}
	for i := range newPos {
		newPos[i] *= math.Pi / 180
	}
	m.SetPosition(newPos)

	solved = ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)
}
