package universalrobots

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/test"

	"go.viam.com/core/arm"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
)

func testUR5eForwardKinements(t *testing.T, jointRadians []float64, correct *pb.ArmPosition) {
	m, err := kinematics.ParseJSON(ur5modeljson)
	test.That(t, err, test.ShouldBeNil)

	pos := kinematics.ComputePosition(m, arm.JointPositionsFromRadians(jointRadians))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.X, test.ShouldAlmostEqual, correct.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, correct.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, correct.Z, .01)

	fromDH := computeUR5ePosition(jointRadians)
	test.That(t, pos.X, test.ShouldAlmostEqual, fromDH.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, fromDH.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, fromDH.Z, .01)

	test.That(t, pos.OX, test.ShouldAlmostEqual, fromDH.OX, .01)
	test.That(t, pos.OY, test.ShouldAlmostEqual, fromDH.OY, .01)
	test.That(t, pos.OZ, test.ShouldAlmostEqual, fromDH.OZ, .01)

	// TODO(erh): make this test work
	//test.That(t, pos.Theta, test.ShouldAlmostEqual, fromDH.Theta, .01)

}

func testUR5eInverseKinements(t *testing.T, pos *pb.ArmPosition) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	m, err := kinematics.ParseJSON(ur5modeljson)
	test.That(t, err, test.ShouldBeNil)
	ik := kinematics.CreateCombinedIKSolver(m, logger, 4)

	solution, err := ik.Solve(ctx, pos, arm.JointPositionsFromRadians([]float64{0, 0, 0, 0, 0, 0}))
	test.That(t, err, test.ShouldBeNil)

	// we test that if we go forward from these joints, we end up in the same place
	jointRadians := arm.JointPositionsToRadians(solution)
	fromDH := computeUR5ePosition(jointRadians)
	test.That(t, pos.X, test.ShouldAlmostEqual, fromDH.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, fromDH.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, fromDH.Z, .01)
}

func TestKin1(t *testing.T) {
	// data came from excel file found here
	// https://www.universal-robots.com/articles/ur/application-installation/dh-parameters-for-calculations-of-kinematics-and-dynamics/
	// https://s3-eu-west-1.amazonaws.com/ur-support-site/45257/DH-Transformation.xlsx
	// Note: we use millimeters, they use meters

	// Section 1 - first we test each joint independently

	//    Home
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, 0}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})

	//    Joint 0
	testUR5eForwardKinements(t, []float64{math.Pi / 2, 0, 0, 0, 0, 0}, &pb.ArmPosition{X: 232.90, Y: -817.2, Z: 62.80})
	testUR5eForwardKinements(t, []float64{math.Pi, 0, 0, 0, 0, 0}, &pb.ArmPosition{X: 817.2, Y: 232.90, Z: 62.80})

	//    Joint 1
	testUR5eForwardKinements(t, []float64{0, math.Pi / -2, 0, 0, 0, 0}, &pb.ArmPosition{X: -99.7, Y: -232.90, Z: 979.70})
	testUR5eForwardKinements(t, []float64{0, math.Pi / 2, 0, 0, 0, 0}, &pb.ArmPosition{X: 99.7, Y: -232.90, Z: -654.70})
	testUR5eForwardKinements(t, []float64{0, math.Pi, 0, 0, 0, 0}, &pb.ArmPosition{X: 817.2, Y: -232.90, Z: 262.2})

	//    Joint 2
	testUR5eForwardKinements(t, []float64{0, 0, math.Pi / 2, 0, 0, 0}, &pb.ArmPosition{X: -325.3, Y: -232.90, Z: -229.7})
	testUR5eForwardKinements(t, []float64{0, 0, math.Pi, 0, 0, 0}, &pb.ArmPosition{X: -32.8, Y: -232.90, Z: 262.2})

	//    Joint 3
	testUR5eForwardKinements(t, []float64{0, 0, 0, math.Pi / 2, 0, 0}, &pb.ArmPosition{X: -717.5, Y: -232.90, Z: 162.5})
	testUR5eForwardKinements(t, []float64{0, 0, 0, math.Pi, 0, 0}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 262.2})

	//    Joint 4
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, math.Pi / 2, 0}, &pb.ArmPosition{X: -916.80, Y: -133.3, Z: 62.8})
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, math.Pi, 0}, &pb.ArmPosition{X: -817.2, Y: -33.7, Z: 62.8})

	//    Joint 5
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, math.Pi / 2}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, math.Pi}, &pb.ArmPosition{X: -817.2, Y: -232.90, Z: 62.80})

	// Section 2 - try some consistent angle
	rad := math.Pi / 4
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: 16.62, Y: -271.49, Z: -509.52})

	rad = math.Pi / 2
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: 133.3, Y: 292.5, Z: -162.9})

	rad = math.Pi
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &pb.ArmPosition{X: -32.8, Y: 33.7, Z: 262.2})

	// Section 3 - try some random angles
	testUR5eForwardKinements(t, []float64{math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2, 0}, &pb.ArmPosition{X: 193.91, Y: 5.39, Z: -654.63})
	testUR5eForwardKinements(t, []float64{0, math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2}, &pb.ArmPosition{X: 97.11, Y: -203.73, Z: -394.65})

	testUR5eInverseKinements(t,
		&pb.ArmPosition{X: -202.31, Y: -577.75, Z: 318.58, Theta: 51.84, OX: 0.47, OY: -.42, OZ: -.78},
	)
}

type dhConstants struct {
	a, d, alpha float64
}

func (d dhConstants) matrix(theta float64) *mat.Dense {
	m := mat.NewDense(4, 4, nil)

	m.Set(0, 0, math.Cos(theta))
	m.Set(0, 1, -1*math.Sin(theta)*math.Cos(d.alpha))
	m.Set(0, 2, math.Sin(theta)*math.Sin(d.alpha))
	m.Set(0, 3, d.a*math.Cos(theta))

	m.Set(1, 0, math.Sin(theta))
	m.Set(1, 1, math.Cos(theta)*math.Cos(d.alpha))
	m.Set(1, 2, -1*math.Cos(theta)*math.Sin(d.alpha))
	m.Set(1, 3, d.a*math.Sin(theta))

	m.Set(2, 0, 0)
	m.Set(2, 1, math.Sin(d.alpha))
	m.Set(2, 2, math.Cos(d.alpha))
	m.Set(2, 3, d.d)

	m.Set(3, 3, 1)

	return m
}

var jointConstants = []dhConstants{
	{0.0000, 0.1625, math.Pi / 2},
	{-0.4250, 0.0000, 0},
	{-0.3922, 0.0000, 0},
	{0.0000, 0.1333, math.Pi / 2},
	{0.0000, 0.0997, -1 * math.Pi / 2},
	{0.0000, 0.0996, 0},
}

var orientationDH = dhConstants{0, 1, math.Pi / -2}

func computeUR5ePosition(jointRadians []float64) *pb.ArmPosition {
	res := jointConstants[0].matrix(jointRadians[0])
	for x, theta := range jointRadians {
		if x == 0 {
			continue
		}

		temp := mat.NewDense(4, 4, nil)
		temp.Mul(res, jointConstants[x].matrix(theta))
		res = temp
	}

	var o mat.Dense
	o.Mul(res, orientationDH.matrix(0))

	ov := spatialmath.OrientationVec{
		OX: o.At(0, 3) - res.At(0, 3),
		OY: o.At(1, 3) - res.At(1, 3),
		OZ: o.At(2, 3) - res.At(2, 3),
		//Theta: utils.RadToDeg(math.Acos(o.At(0,0))), // TODO(erh): fix this
	}
	ov.Normalize()

	return &pb.ArmPosition{
		X:  1000 * res.At(0, 3),
		Y:  1000 * res.At(1, 3),
		Z:  1000 * res.At(2, 3),
		OX: ov.OX,
		OY: ov.OY,
		OZ: ov.OZ,
		//Theta: ov.Theta,
	}

}
