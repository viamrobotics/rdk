package kinematics

import (
	//~ "fmt"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.viam.com/robotcore/kinematics/kinmath"
	"go.viam.com/robotcore/utils"
	//~ "gonum.org/v1/gonum/num/dualquat"
	//~ "gonum.org/v1/gonum/num/quat"
)

type Position struct {
	X, Y, Z float64 // millimeters distance of the end effector from the base

	Rx, Ry, Rz float64 // angular orientation about each axis, in degrees
}

func TestCreateJacIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	ik := CreateJacobianIKSolver(m)
	m.SetPosition([]float64{1, 0, 0, 0, 0, 1})
	m.ForwardPosition()

	pos := Position{360, 0, 360.25, 15, 0, 0}
	pos.Rx *= math.Pi / 180
	pos.Ry *= math.Pi / 180
	pos.Rz *= math.Pi / 180

	transform := kinmath.NewQuatTransFromRotation(pos.Rx, pos.Ry, pos.Rz)
	transform.SetX(pos.X / 2)
	transform.SetY(pos.Y / 2)
	transform.SetZ(pos.Z / 2)

	ik.AddGoal(transform, 0)
	solved := ik.Solve()
	test.That(t, solved, test.ShouldBeTrue)

	// TODO(pl): Below is a more difficult IK problem which as of the writing of this comment Jac IK is not able to solve
	//~ pos = Position{-66.445827416798814, -133.99229347583582, 372.4849299627513, -178.88747811107424, -33.160094626838045, -111.02282693533935}
	//~ pos.Rx *= math.Pi/180
	//~ pos.Ry *= math.Pi/180
	//~ pos.Rz *= math.Pi/180

	//~ transform = kinmath.NewQuatTransFromRotation(pos.Rx, pos.Ry, pos.Rz)
	//~ transform.SetQuat(dualquat.Number{quat.Number{0.23488388003361693,0.5520144509489663,-0.7833317466881079,0.16279122665065213}, quat.Number{}})
	//~ transform.SetX(pos.X/2)
	//~ transform.SetY(pos.Y/2)
	//~ transform.SetZ(pos.Z/2)
	//~ fmt.Println("goal", transform)

	//~ ik.AddGoal(transform, 0)

	//~ newPos := []float64{69.35309996071989, 28.752097952708045, -101.57720046840646, 0.9393597585332618, -73.96221972947882, 0.03845332136188379}
	//~ for i, _ := range(newPos){
	//~ newPos[i] *= math.Pi/180
	//~ }
	//~ m.SetPosition(newPos)
	//~ m.ForwardPosition()

	//~ fmt.Println("start", m.GetOperationalPosition(0))
	//~ fmt.Println("start 6d", m.Get6dPosition(0))
	//~ solved = ik.Solve()
	//~ test.That(t, solved, test.ShouldBeTrue)
}
