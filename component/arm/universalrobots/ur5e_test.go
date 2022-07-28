package universalrobots

import (
	"context"
	"math"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"github.com/go-gl/mathgl/mgl64"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestUR5eForwardKinementsSVAvsDH(t *testing.T) {
	numTests := 10000

	mSVA, err := referenceframe.UnmarshalModelJSON(ur5modeljson, "")
	test.That(t, err, test.ShouldBeNil)
	mDH, err := referenceframe.UnmarshalModelJSON(ur5DHmodeljson, "")
	test.That(t, err, test.ShouldBeNil)

	seed := rand.New(rand.NewSource(23))
	for i := 0; i < numTests; i++ {
		joints := referenceframe.JointPositionsFromRadians(referenceframe.GenerateRandomConfiguration(mSVA, seed))

		posSVA, err := motionplan.ComputePosition(mSVA, joints)
		test.That(t, err, test.ShouldBeNil)
		posDH, err := motionplan.ComputePosition(mDH, joints)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, posSVA.X, test.ShouldAlmostEqual, posDH.X, .01)
		test.That(t, posSVA.Y, test.ShouldAlmostEqual, posDH.Y, .01)
		test.That(t, posSVA.Z, test.ShouldAlmostEqual, posDH.Z, .01)

		test.That(t, posSVA.OX, test.ShouldAlmostEqual, posDH.OX, .01)
		test.That(t, posSVA.OY, test.ShouldAlmostEqual, posDH.OY, .01)
		test.That(t, posSVA.OZ, test.ShouldAlmostEqual, posDH.OZ, .01)
		test.That(t, posSVA.Theta, test.ShouldAlmostEqual, posDH.Theta, .01)
	}
}

func testUR5eForwardKinements(t *testing.T, jointRadians []float64, correct *commonpb.Pose) {
	t.Helper()
	m, err := referenceframe.UnmarshalModelJSON(ur5modeljson, "")
	test.That(t, err, test.ShouldBeNil)

	pos, err := motionplan.ComputePosition(m, referenceframe.JointPositionsFromRadians(jointRadians))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.X, test.ShouldAlmostEqual, correct.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, correct.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, correct.Z, .01)

	fromDH := computeUR5ePosition(t, jointRadians)
	test.That(t, pos.X, test.ShouldAlmostEqual, fromDH.X, .01)
	test.That(t, pos.Y, test.ShouldAlmostEqual, fromDH.Y, .01)
	test.That(t, pos.Z, test.ShouldAlmostEqual, fromDH.Z, .01)

	test.That(t, pos.OX, test.ShouldAlmostEqual, fromDH.OX, .01)
	test.That(t, pos.OY, test.ShouldAlmostEqual, fromDH.OY, .01)
	test.That(t, pos.OZ, test.ShouldAlmostEqual, fromDH.OZ, .01)
	test.That(t, pos.Theta, test.ShouldAlmostEqual, fromDH.Theta, .01)
}

func testUR5eInverseKinements(t *testing.T, pos *commonpb.Pose) {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	m, err := referenceframe.UnmarshalModelJSON(ur5modeljson, "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := motionplan.NewCBiRRTMotionPlanner(m, 4, rand.New(rand.NewSource(1)), logger)
	test.That(t, err, test.ShouldBeNil)

	steps, err := mp.Plan(ctx, pos, referenceframe.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0}), nil)
	test.That(t, err, test.ShouldBeNil)
	solution := steps[len(steps)-1]

	// we test that if we go forward from these joints, we end up in the same place
	jointRadians := referenceframe.InputsToFloats(solution)
	fromDH := computeUR5ePosition(t, jointRadians)
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
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, 0}, &commonpb.Pose{X: -817.2, Y: -232.90, Z: 62.80})

	//    Joint 0
	testUR5eForwardKinements(t, []float64{math.Pi / 2, 0, 0, 0, 0, 0}, &commonpb.Pose{X: 232.90, Y: -817.2, Z: 62.80})
	testUR5eForwardKinements(t, []float64{math.Pi, 0, 0, 0, 0, 0}, &commonpb.Pose{X: 817.2, Y: 232.90, Z: 62.80})

	//    Joint 1
	testUR5eForwardKinements(t, []float64{0, math.Pi / -2, 0, 0, 0, 0}, &commonpb.Pose{X: -99.7, Y: -232.90, Z: 979.70})
	testUR5eForwardKinements(t, []float64{0, math.Pi / 2, 0, 0, 0, 0}, &commonpb.Pose{X: 99.7, Y: -232.90, Z: -654.70})
	testUR5eForwardKinements(t, []float64{0, math.Pi, 0, 0, 0, 0}, &commonpb.Pose{X: 817.2, Y: -232.90, Z: 262.2})

	//    Joint 2
	testUR5eForwardKinements(t, []float64{0, 0, math.Pi / 2, 0, 0, 0}, &commonpb.Pose{X: -325.3, Y: -232.90, Z: -229.7})
	testUR5eForwardKinements(t, []float64{0, 0, math.Pi, 0, 0, 0}, &commonpb.Pose{X: -32.8, Y: -232.90, Z: 262.2})

	//    Joint 3
	testUR5eForwardKinements(t, []float64{0, 0, 0, math.Pi / 2, 0, 0}, &commonpb.Pose{X: -717.5, Y: -232.90, Z: 162.5})
	testUR5eForwardKinements(t, []float64{0, 0, 0, math.Pi, 0, 0}, &commonpb.Pose{X: -817.2, Y: -232.90, Z: 262.2})

	//    Joint 4
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, math.Pi / 2, 0}, &commonpb.Pose{X: -916.80, Y: -133.3, Z: 62.8})
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, math.Pi, 0}, &commonpb.Pose{X: -817.2, Y: -33.7, Z: 62.8})

	//    Joint 5
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, math.Pi / 2}, &commonpb.Pose{X: -817.2, Y: -232.90, Z: 62.80})
	testUR5eForwardKinements(t, []float64{0, 0, 0, 0, 0, math.Pi}, &commonpb.Pose{X: -817.2, Y: -232.90, Z: 62.80})

	// Section 2 - try some consistent angle
	rad := math.Pi / 4
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &commonpb.Pose{X: 16.62, Y: -271.49, Z: -509.52})

	rad = math.Pi / 2
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &commonpb.Pose{X: 133.3, Y: 292.5, Z: -162.9})

	rad = math.Pi
	testUR5eForwardKinements(t, []float64{rad, rad, rad, rad, rad, rad}, &commonpb.Pose{X: -32.8, Y: 33.7, Z: 262.2})

	// Section 3 - try some random angles
	testUR5eForwardKinements(t,
		[]float64{math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2, 0},
		&commonpb.Pose{X: 193.91, Y: 5.39, Z: -654.63})
	testUR5eForwardKinements(t,
		[]float64{0, math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2},
		&commonpb.Pose{X: 97.11, Y: -203.73, Z: -394.65})

	testUR5eInverseKinements(t,
		&commonpb.Pose{X: -202.31, Y: -577.75, Z: 318.58, Theta: 51.84, OX: 0.47, OY: -.42, OZ: -.78},
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

func computeUR5ePosition(t *testing.T, jointRadians []float64) *commonpb.Pose {
	t.Helper()
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

	ov := spatialmath.OrientationVector{
		OX: o.At(0, 3) - res.At(0, 3),
		OY: o.At(1, 3) - res.At(1, 3),
		OZ: o.At(2, 3) - res.At(2, 3),
		// Theta: utils.RadToDeg(math.Acos(o.At(0,0))), // TODO(erh): fix this
	}
	ov.Normalize()

	resMgl := mgl64.Ident4()
	// Copy to a mgl64 4x4 to convert to quaternion
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			resMgl.Set(r, c, res.At(r, c))
		}
	}
	q := mgl64.Mat4ToQuat(resMgl)
	poseOV := spatialmath.QuatToOV(quat.Number{q.W, q.X(), q.Y(), q.Z()})

	// Confirm that our matrix -> quaternion -> OV conversion yields the same result as the OV calculated from the DH param
	test.That(t, poseOV.OX, test.ShouldAlmostEqual, ov.OX, .01)
	test.That(t, poseOV.OY, test.ShouldAlmostEqual, ov.OY, .01)
	test.That(t, poseOV.OZ, test.ShouldAlmostEqual, ov.OZ, .01)

	return &commonpb.Pose{
		X:     1000 * res.At(0, 3),
		Y:     1000 * res.At(1, 3),
		Z:     1000 * res.At(2, 3),
		OX:    poseOV.OX,
		OY:    poseOV.OY,
		OZ:    poseOV.OZ,
		Theta: utils.RadToDeg(poseOV.Theta),
	}
}
