package motionplan

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	nCPU = int(math.Max(1.0, float64(runtime.NumCPU()/4)))
)

func poseToSlice(p *commonpb.Pose) []float64 {
	return []float64{p.X, p.Y, p.Z, p.Theta, p.OX, p.OY, p.OZ}
}

// This should test forward kinematics functions.
func TestForwardKinematics(t *testing.T) {
	// Test fake 5DOF arm to confirm kinematics works with non-6dof arms
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 300, 0, 360.25
	expect := []float64{300, 0, 360.25, 0, 1, 0, 0}
	pos, err := ComputePosition(m, &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldBeNil)
	actual := poseToSlice(pos)

	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test the 6dof arm we actually have
	m, err = frame.ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	expect = []float64{365, 0, 360.25, 0, 1, 0, 0}
	pos, err = ComputePosition(m, &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test incorrect joints
	_, err = ComputePosition(m, &pb.JointPositions{Values: []float64{}})
	test.That(t, err, test.ShouldNotBeNil)
	_, err = ComputePosition(m, &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldNotBeNil)

	newPos := []float64{45, -45, 0, 0, 0, 0}
	pos, err = ComputePosition(m, &pb.JointPositions{Values: newPos})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	expect = []float64{57.5, 57.5, 545.1208197765168, 0, 0.5, 0.5, 0.707}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	newPos = []float64{-45, 0, 0, 0, 0, 45}
	pos, err = ComputePosition(m, &pb.JointPositions{Values: newPos})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	expect = []float64{258.0935, -258.0935, 360.25, utils.RadToDeg(0.7854), 0.707, -0.707, 0}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	// Test out of bounds. Note that ComputePosition will return nil on OOB.
	newPos = []float64{-45, 0, 0, 0, 0, 999}
	pos, err = ComputePosition(m, &pb.JointPositions{Values: newPos})
	test.That(t, pos, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}

func floatDelta(l1, l2 []float64) float64 {
	delta := 0.0
	for i, v := range l1 {
		delta += math.Abs(v - l2[i])
	}
	return delta
}

const derivEqualityEpsilon = 1e-16

func derivComponentAlmostEqual(left, right float64) bool {
	return math.Abs(left-right) <= derivEqualityEpsilon
}

func areDerivsEqual(q1, q2 []quat.Number) bool {
	if len(q1) != len(q2) {
		return false
	}
	for i, dq1 := range q1 {
		dq2 := q2[i]
		if !derivComponentAlmostEqual(dq1.Real, dq2.Real) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Imag, dq2.Imag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Jmag, dq2.Jmag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Kmag, dq2.Kmag) {
			return false
		}
	}
	return true
}

func TestDeriv(t *testing.T) {
	// Test identity quaternion
	q := quat.Number{1, 0, 0, 0}
	qDeriv := []quat.Number{{0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}

	match := areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity single-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 0, 0})

	qDeriv = []quat.Number{
		{-0.9092974268256816, -0.4161468365471424, 0, 0},
		{0, 0, 0.4546487134128408, 0},
		{0, 0, 0, 0.4546487134128408},
	}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity multi-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 1.5, 0.2})

	qDeriv = []quat.Number{
		{-0.472134934000233, -0.42654977821280804, -0.4969629339096933, -0.06626172452129245},
		{-0.35410120050017474, -0.4969629339096933, -0.13665473343215354, -0.049696293390969336},
		{-0.0472134934000233, -0.06626172452129245, -0.049696293390969336, 0.22944129454798728},
	}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)
}

// Test dynamic frame systems
// Since kinematics imports reference frame, this needs to be here to avoid circular dependencies.
func TestDynamicFrameSystemXArm(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("test")

	model, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(model, fs.World())

	positions := frame.StartPositions(fs)

	// World point of xArm at 0 position
	poseWorld1 := spatial.NewPoseFromPoint(r3.Vector{207, 0, 112})
	// World point of xArm at (90,-90,90,-90,90,-90) joint positions
	poseWorld2 := spatial.NewPoseFromPoint(r3.Vector{97, -207, -98})

	// Note that because the arm is pointing in a different direction, this point is not a direct inverse of pointWorld2
	pointXarm := spatial.NewPoseFromPoint(r3.Vector{207, 98, -97})

	transformPoint1, err := fs.Transform(positions, frame.NewPoseInFrame("xArm6", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint1.(*frame.PoseInFrame).Pose(), poseWorld1), test.ShouldBeTrue)

	// Test ability to calculate hypothetical out-of-bounds positions for the arm, but still return an error
	positions["xArm6"] = frame.FloatsToInputs(
		[]float64{math.Pi / 2, -math.Pi / 2, math.Pi / 2, -math.Pi / 2, math.Pi / 2, -math.Pi / 2})
	transformPoint2, err := fs.Transform(positions, frame.NewPoseInFrame("xArm6", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint2.(*frame.PoseInFrame).Pose(), poseWorld2), test.ShouldBeTrue)

	transformPoint3, err := fs.Transform(positions, frame.NewPoseInFrame(frame.World, spatial.NewZeroPose()), "xArm6")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint3.(*frame.PoseInFrame).Pose(), pointXarm), test.ShouldBeTrue)
}

// Test a complicated dynamic frame system. We model a UR5 at (100,100,200) holding a camera pointing in line with the
// gripper on a 3cm stick. We also model a xArm6 which is placed on an XY gantry, which is zeroed at (-50,-50,-200).
// Ensure that we are able to transform points from the camera frame into world frame, to gantry frame, and to xarm frame.
func TestComplicatedDynamicFrameSystem(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("test")

	// robot offsets
	urOffset, err := frame.NewStaticFrame("urOffset", spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryXOffset", spatial.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	// build 2 axis gantry manually
	gantryX, err := frame.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryX, gantryOffset)
	gantryY, err := frame.NewTranslationalFrame("gantryY", r3.Vector{0, 1, 0}, frame.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryY, gantryX)

	// xarm on gantry
	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryY)

	// ur5
	modelUR5e, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	// Note that positive Z is always "forwards". If the position of the arm is such that it is pointing elsewhere,
	// the resulting translation will be similarly oriented
	urCamera, err := frame.NewStaticFrame("urCamera", spatial.NewPoseFromPoint(r3.Vector{0, 0, 30}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urCamera, modelUR5e)

	positions := frame.StartPositions(fs)

	poseUR5e := spatial.NewPoseFromPoint(r3.Vector{-717.2, -132.9, 262.8})
	// Camera translates by 30, gripper is pointed at -Y
	poseUR5eCam := spatial.NewPoseFromPoint(r3.Vector{-717.2, -162.9, 262.8})

	poseXarm := spatial.NewPoseFromPoint(r3.Vector{157., -50, -88})
	poseXarmFromCam := spatial.NewPoseFromPoint(r3.Vector{874.2, -112.9, -350.8})

	// Check the UR5e and camera default positions
	transformPoint1, err := fs.Transform(positions, frame.NewPoseInFrame("UR5e", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	transformPoint2, err := fs.Transform(positions, frame.NewPoseInFrame("urCamera", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	transformPoint3, err := fs.Transform(positions, frame.NewPoseInFrame("xArm6", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	transformPoint4, err := fs.Transform(positions, frame.NewPoseInFrame("urCamera", spatial.NewZeroPose()), "xArm6")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint1.(*frame.PoseInFrame).Pose(), poseUR5e), test.ShouldBeTrue)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint2.(*frame.PoseInFrame).Pose(), poseUR5eCam), test.ShouldBeTrue)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint3.(*frame.PoseInFrame).Pose(), poseXarm), test.ShouldBeTrue)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint4.(*frame.PoseInFrame).Pose(), poseXarmFromCam), test.ShouldBeTrue)

	// Move the UR5e so its local Z axis is pointing approximately towards the xArm (at positive X)
	positions["UR5e"] = frame.FloatsToInputs([]float64{0, 0, 0, 0, -math.Pi / 2, -math.Pi / 2})

	// A point that is 813.6, -50, 200 from the camera
	// This puts the point in the Z plane of the xArm6
	targetPoint := spatial.NewPoseFromPoint(r3.Vector{350.8, -50, 200})
	// Target point in world
	tf, err := fs.Transform(positions, frame.NewPoseInFrame("urCamera", targetPoint), frame.World)
	test.That(t, err, test.ShouldBeNil)
	worldPointLoc := spatial.NewPoseFromPoint(tf.(*frame.PoseInFrame).Pose().Point())

	// Move the XY gantry such that the xArm6 is now at the point specified
	positions["gantryX"] = frame.FloatsToInputs([]float64{worldPointLoc.Point().X - poseXarm.Point().X})
	positions["gantryY"] = frame.FloatsToInputs([]float64{worldPointLoc.Point().Y - poseXarm.Point().Y})

	// Confirm the xArm6 is now at the same location as the point
	newPointXarm, err := fs.Transform(positions, frame.NewPoseInFrame("xArm6", spatial.NewZeroPose()), frame.World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(newPointXarm.(*frame.PoseInFrame).Pose(), worldPointLoc), test.ShouldBeTrue)

	// If the above passes, then converting one directly to the other should be (0,0,0)
	pointCamToXarm, err := fs.Transform(positions, frame.NewPoseInFrame("urCamera", targetPoint), "xArm6")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(pointCamToXarm.(*frame.PoseInFrame).Pose(), spatial.NewZeroPose()), test.ShouldBeTrue)
}

func TestCombinedIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	// Test ability to arrive at another position
	pos := &commonpb.Pose{
		X:  -46,
		Y:  -133,
		Z:  372,
		OX: 1.79,
		OY: -1.32,
		OZ: -1.11,
	}
	solution, err := solveTest(context.Background(), ik, pos, home)
	test.That(t, err, test.ShouldBeNil)

	// Test moving forward 20 in X direction from previous position
	pos = &commonpb.Pose{
		X:  -66,
		Y:  -133,
		Z:  372,
		OX: 1.78,
		OY: -3.3,
		OZ: -1.11,
	}
	_, err = solveTest(context.Background(), ik, pos, solution[0])
	test.That(t, err, test.ShouldBeNil)
}

func TestUR5NloptIKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)

	goalJP := frame.JointPositionsFromRadians([]float64{-4.128, 2.71, 2.798, 2.3, 1.291, 0.62})
	goal, err := ComputePosition(m, goalJP)
	test.That(t, err, test.ShouldBeNil)
	_, err = solveTest(context.Background(), ik, goal, home)
	test.That(t, err, test.ShouldBeNil)
}

func TestSVAvsDH(t *testing.T) {
	mSVA, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mDH, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/universalrobots/ur5e_DH.json"), "")
	test.That(t, err, test.ShouldBeNil)

	numTests := 10000

	seed := rand.New(rand.NewSource(23))
	for i := 0; i < numTests; i++ {
		joints := mSVA.ProtobufFromInput(frame.RandomFrameInputs(mSVA, seed))

		posSVA, err := ComputePosition(mSVA, joints)
		test.That(t, err, test.ShouldBeNil)
		posDH, err := ComputePosition(mDH, joints)
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

func TestCombinedCPUs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	ik, err := CreateCombinedIKSolver(m, logger, runtime.NumCPU()/400000)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(ik.solvers), test.ShouldEqual, 1)
}

func solveTest(ctx context.Context,
	solver InverseKinematics,
	goal *commonpb.Pose,
	seed []frame.Input,
) ([][]frame.Input, error) {
	goalPos := spatial.NewPoseFromProtobuf(goal)

	solutionGen := make(chan []frame.Input)
	ikErr := make(chan error)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Spawn the IK solver to generate solutions until done
	go func() {
		defer close(ikErr)
		ikErr <- solver.Solve(ctxWithCancel, solutionGen, goalPos, seed, NewSquaredNormMetric())
	}()

	var solutions [][]frame.Input

	// Solve the IK solver. Loop labels are required because `break` etc in a `select` will break only the `select`.
IK:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		select {
		case step := <-solutionGen:
			solutions = append(solutions, step)
			// Skip the return check below until we have nothing left to read from solutionGen
			continue IK
		default:
		}

		select {
		case <-ikErr:
			// If we have a return from the IK solver, there are no more solutions, so we finish processing above
			// until we've drained the channel
			break IK
		default:
		}
	}
	cancel()
	if len(solutions) == 0 {
		return nil, errors.New("unable to solve for position")
	}

	return solutions, nil
}
