package kinematics

import (
	"math"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"go.viam.com/test"
)

func TestCreateJacIKSolver(t *testing.T) {
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(m)
	m.SetPosition([]float64{1, 0, 0, 0, 0, 1})
	m.ForwardPosition()

	pos := pb.ArmPosition{X: 360, Y: 0, Z: 360, OX: 1, OY: 0, OZ: 0, Theta: 15}
	pos.Theta *= math.Pi / 180

	transform := spatialmath.NewDualQuaternionFromArmPos(&pos)

	ik.AddGoal(transform, 0)
	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)

	// TODO(pl): Below is a more difficult IK problem which as of the writing of this comment Jac IK is not able to solve
	// pos = Position{-66.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	// pos.Rx *= math.Pi/180
	// pos.Ry *= math.Pi/180
	// pos.Rz *= math.Pi/180

	// transform = spatialmath.NewQuatTransFromRotation(pos.Rx, pos.Ry, pos.Rz)
	// transform.SetQuat(dualquat.Number{quat.Number{0.23488388003361693,0.5520144509489663,-0.7833317466881079,0.16279122665065213}, quat.Number{}})
	// transform.SetX(pos.X/2)
	// transform.SetY(pos.Y/2)
	// transform.SetZ(pos.Z/2)
	// fmt.Println("goal", transform)

	// ik.AddGoal(transform, 0)

	// newPos := []float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379}
	// for i, _ := range(newPos){
	// newPos[i] *= math.Pi/180
	// }
	// m.SetPosition(newPos)
	// m.ForwardPosition()

	// fmt.Println("start", m.GetOperationalPosition(0))
	// fmt.Println("start 6d", m.Get6dPosition(0))
	// solved = ik.Solve()
	// test.That(t, solved, test.ShouldBeTrue)
}
