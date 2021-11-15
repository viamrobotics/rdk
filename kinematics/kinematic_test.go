package kinematics

import (
	"math"
	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/component/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"
)

func poseToSlice(p *commonpb.Pose) []float64 {
	return []float64{p.X, p.Y, p.Z, p.Theta, p.OX, p.OY, p.OZ}
}

// This should test forward kinematics functions
func TestForwardKinematics(t *testing.T) {
	// Test fake 5DOF arm to confirm kinematics works with non-6dof arms
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 300, 0, 360.25
	expect := []float64{300, 0, 360.25, 0, 1, 0, 0}
	pos, err := ComputePosition(m, &pb.ArmJointPositions{Degrees: []float64{0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldBeNil)
	actual := poseToSlice(pos)

	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test the 6dof arm we actually have
	m, err = frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	expect = []float64{365, 0, 360.25, 0, 1, 0, 0}
	pos, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test incorrect joints
	_, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: []float64{}})
	test.That(t, err, test.ShouldNotBeNil)
	_, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0, 0}})
	test.That(t, err, test.ShouldNotBeNil)

	newPos := []float64{45, -45, 0, 0, 0, 0}
	pos, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: newPos})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	expect = []float64{57.5, 57.5, 545.1208197765168, 0, 0.5, 0.5, 0.707}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	newPos = []float64{-45, 0, 0, 0, 0, 45}
	pos, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: newPos})
	test.That(t, err, test.ShouldBeNil)
	actual = poseToSlice(pos)
	expect = []float64{258.0935, -258.0935, 360.25, utils.RadToDeg(0.7854), 0.707, -0.707, 0}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	// Test out of bounds
	newPos = []float64{-45, 0, 0, 0, 0, 999}
	pos, err = ComputePosition(m, &pb.ArmJointPositions{Degrees: newPos})
	test.That(t, pos, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestSwingEdgeCases(t *testing.T) {
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	origin := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0})
	oob := frame.FloatsToInputs([]float64{0, 0, 0, 0, 999})
	swing, err := calcSwingAmount(oob, origin, m)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, swing, test.ShouldEqual, math.Inf(1))
	swing, err = calcSwingAmount(origin, oob, m)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, swing, test.ShouldEqual, math.Inf(1))
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

	qDeriv = []quat.Number{{-0.9092974268256816, -0.4161468365471424, 0, 0},
		{0, 0, 0.4546487134128408, 0},
		{0, 0, 0, 0.4546487134128408}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity multi-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 1.5, 0.2})

	qDeriv = []quat.Number{{-0.472134934000233, -0.42654977821280804, -0.4969629339096933, -0.06626172452129245},
		{-0.35410120050017474, -0.4969629339096933, -0.13665473343215354, -0.049696293390969336},
		{-0.0472134934000233, -0.06626172452129245, -0.049696293390969336, 0.22944129454798728}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)
}

// Test dynamic frame systems
// Since kinematics imports reference frame, this needs to be here to avoid circular dependencies
func TestDynamicFrameSystemXArm(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("test")

	model, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(model, fs.World())

	positions := frame.StartPositions(fs)

	// World point of xArm at 0 position
	pointWorld1 := r3.Vector{207, 0, 112}
	// World point of xArm at (90,-90,90,-90,90,-90) joint positions
	pointWorld2 := r3.Vector{97, -207, -98}

	// Note that because the arm is pointing in a different direction, this point is not a direct inverse of pointWorld2
	pointXarm := r3.Vector{207, 98, -97}

	transformPoint1, err := fs.TransformFrame(positions, fs.GetFrame("xArm6"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.Point().X, test.ShouldAlmostEqual, pointWorld1.X)
	test.That(t, transformPoint1.Point().Y, test.ShouldAlmostEqual, pointWorld1.Y)
	test.That(t, transformPoint1.Point().Z, test.ShouldAlmostEqual, pointWorld1.Z)

	// Test ability to calculate hypothetical out-of-bounds positions for the arm, but still return an error
	positions["xArm6"] = frame.FloatsToInputs([]float64{math.Pi / 2, -math.Pi / 2, math.Pi / 2, -math.Pi / 2, math.Pi / 2, -math.Pi / 2})
	transformPoint2, err := fs.TransformFrame(positions, fs.GetFrame("xArm6"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, transformPoint2.Point().X, test.ShouldAlmostEqual, pointWorld2.X)
	test.That(t, transformPoint2.Point().Y, test.ShouldAlmostEqual, pointWorld2.Y)
	test.That(t, transformPoint2.Point().Z, test.ShouldAlmostEqual, pointWorld2.Z)

	transformPoint3, err := fs.TransformFrame(positions, fs.GetFrame(frame.World), fs.GetFrame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint3.Point().X, test.ShouldAlmostEqual, pointXarm.X)
	test.That(t, transformPoint3.Point().Y, test.ShouldAlmostEqual, pointXarm.Y)
	test.That(t, transformPoint3.Point().Z, test.ShouldAlmostEqual, pointXarm.Z)
}

// Test a complicated dynamic frame system. We model a UR5 at (100,100,200) holding a camera pointing in line with the
// gripper on a 3cm stick. We also model a xArm6 which is placed on an XY gantry, which is zeroed at (-50,-50,-200).
// Ensure that we are able to transform points from the camera frame into world frame, to gantry frame, and to xarm frame.
func TestComplicatedDynamicFrameSystem(t *testing.T) {
	fs := frame.NewEmptySimpleFrameSystem("test")

	urOffset, err := frame.NewStaticFrame("urOffset", spatial.NewPoseFromPoint(r3.Vector{100, 100, 200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urOffset, fs.World())
	gantryOffset, err := frame.NewStaticFrame("gantryOffset", spatial.NewPoseFromPoint(r3.Vector{-50, -50, -200}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	limits := []frame.Limit{{math.Inf(-1), math.Inf(1)}, {math.Inf(-1), math.Inf(1)}}

	gantry, err := frame.NewTranslationalFrame("gantry", []bool{true, true, false}, limits)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantry, gantryOffset)

	modelXarm, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantry)

	modelUR5e, err := frame.ParseJSONFile(utils.ResolveFile("robots/universalrobots/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelUR5e, urOffset)

	// Note that positive Z is always "forwards". If the position of the arm is such that it is pointing elsewhere,
	// the resulting translation will be similarly oriented
	urCamera, err := frame.NewStaticFrame("urCamera", spatial.NewPoseFromPoint(r3.Vector{0, 0, 30}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(urCamera, modelUR5e)

	positions := frame.StartPositions(fs)

	pointUR5e := r3.Vector{-717.2, -132.9, 262.8}
	// Camera translates by 30, gripper is pointed at -Y
	pointUR5eCam := r3.Vector{-717.2, -162.9, 262.8}

	pointXarm := r3.Vector{157., -50, -88}
	pointXarmFromCam := r3.Vector{874.2, -112.9, -350.8}

	// Check the UR5e and camera default positions
	transformPoint1, err := fs.TransformFrame(positions, fs.GetFrame("UR5e"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	transformPoint2, err := fs.TransformFrame(positions, fs.GetFrame("urCamera"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	transformPoint3, err := fs.TransformFrame(positions, fs.GetFrame("xArm6"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	transformPoint4, err := fs.TransformFrame(positions, fs.GetFrame("urCamera"), fs.GetFrame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint1.Point().X, test.ShouldAlmostEqual, pointUR5e.X)
	test.That(t, transformPoint1.Point().Y, test.ShouldAlmostEqual, pointUR5e.Y)
	test.That(t, transformPoint1.Point().Z, test.ShouldAlmostEqual, pointUR5e.Z)
	test.That(t, transformPoint2.Point().X, test.ShouldAlmostEqual, pointUR5eCam.X)
	test.That(t, transformPoint2.Point().Y, test.ShouldAlmostEqual, pointUR5eCam.Y)
	test.That(t, transformPoint2.Point().Z, test.ShouldAlmostEqual, pointUR5eCam.Z)
	test.That(t, transformPoint3.Point().X, test.ShouldAlmostEqual, pointXarm.X)
	test.That(t, transformPoint3.Point().Y, test.ShouldAlmostEqual, pointXarm.Y)
	test.That(t, transformPoint3.Point().Z, test.ShouldAlmostEqual, pointXarm.Z)
	test.That(t, transformPoint4.Point().X, test.ShouldAlmostEqual, pointXarmFromCam.X)
	test.That(t, transformPoint4.Point().Y, test.ShouldAlmostEqual, pointXarmFromCam.Y)
	test.That(t, transformPoint4.Point().Z, test.ShouldAlmostEqual, pointXarmFromCam.Z)

	// Move the UR5e so its local Z axis is pointing approximately towards the xArm (at positive X)
	positions["UR5e"] = frame.FloatsToInputs([]float64{0, 0, 0, 0, -math.Pi / 2, -math.Pi / 2})

	// A point that is 813.6, -50, 200 from the camera
	// This puts the point in the Z plane of the xArm6
	targetPoint := r3.Vector{350.8, -50, 200}
	// Target point in world
	worldPointLoc, err := fs.TransformPoint(positions, targetPoint, fs.GetFrame("urCamera"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)

	// Move the XY gantry such that the xArm6 is now at the point specified
	positions["gantry"] = frame.FloatsToInputs([]float64{worldPointLoc.X - pointXarm.X, worldPointLoc.Y - pointXarm.Y})

	// Confirm the xArm6 is now at the same location as the point
	newPointXarm, err := fs.TransformFrame(positions, fs.GetFrame("xArm6"), fs.GetFrame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newPointXarm.Point().X, test.ShouldAlmostEqual, worldPointLoc.X)
	test.That(t, newPointXarm.Point().Y, test.ShouldAlmostEqual, worldPointLoc.Y)
	test.That(t, newPointXarm.Point().Z, test.ShouldAlmostEqual, worldPointLoc.Z)

	// If the above passes, then converting one directly to the other should be (0,0,0)
	pointCamToXarm, err := fs.TransformPoint(positions, targetPoint, fs.GetFrame("urCamera"), fs.GetFrame("xArm6"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pointCamToXarm.X, test.ShouldAlmostEqual, 0)
	test.That(t, pointCamToXarm.Y, test.ShouldAlmostEqual, 0)
	test.That(t, pointCamToXarm.Z, test.ShouldAlmostEqual, 0)
}

func TestFixOvIncrement(t *testing.T) {
	pos1 := &commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	pos2 := &commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	// Increment, but we're not pointing at Z axis, so should do nothing
	pos2.OX = -0.1
	outpos := fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)

	// point at positive Z axis, decrement OX, should subtract 180
	pos1.OZ = 1
	pos2.OZ = 1
	pos1.OY = 0
	pos2.OY = 0
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos.Theta, test.ShouldEqual, -165)

	// Spatial translation is incremented, should do nothing
	pos2.X -= 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)

	// Point at -Z, increment OY
	pos2.X += 0.1
	pos2.OX += 0.1
	pos1.OZ = -1
	pos2.OZ = -1
	pos2.OY = 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos.Theta, test.ShouldEqual, 105)

	// OX and OY are both incremented, should do nothing
	pos2.OX += 0.1
	outpos = fixOvIncrement(pos2, pos1)
	test.That(t, outpos, test.ShouldResemble, pos2)
}
