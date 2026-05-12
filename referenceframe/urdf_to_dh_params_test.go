package referenceframe

import (
	"encoding/xml"
	"math"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

const ur5eTolerance = 1e-6

// expectedUR5eDHParams are the classic UR5e Denavit-Hartenberg parameters in URDFToDHParams's
// SI output units (meters for A/D, radians for Alpha/Theta/Min/Max). Source: Universal
// Robots kinematic calibration documentation. Limits match testfiles/ur5e-real.urdf.
// These are what URDFToDHParams must produce when given testfiles/ur5e-real.urdf.
var expectedUR5eDHParams = []DHParamConfig{
	{ID: "shoulder_pan_joint", Parent: "world", D: 0.1625, Theta: 0, A: 0, Alpha: math.Pi / 2, Min: -2 * math.Pi, Max: 2 * math.Pi},
	{ID: "shoulder_lift_joint", Parent: "shoulder_pan_joint", D: 0, Theta: 0, A: -0.425, Alpha: 0, Min: -2 * math.Pi, Max: 2 * math.Pi},
	{ID: "elbow_joint", Parent: "shoulder_lift_joint", D: 0, Theta: 0, A: -0.3922, Alpha: 0, Min: -math.Pi, Max: math.Pi},
	{ID: "wrist_1_joint", Parent: "elbow_joint", D: 0.1333, Theta: 0, A: 0, Alpha: math.Pi / 2, Min: -2 * math.Pi, Max: 2 * math.Pi},
	{ID: "wrist_2_joint", Parent: "wrist_1_joint", D: 0.0997, Theta: 0, A: 0, Alpha: -math.Pi / 2, Min: -2 * math.Pi, Max: 2 * math.Pi},
	{ID: "wrist_3_joint", Parent: "wrist_2_joint", D: 0.0996, Theta: 0, A: 0, Alpha: 0, Min: -2 * math.Pi, Max: 2 * math.Pi},
}

// loadURDF is a small test helper that reads a URDF file and unmarshals it
// into a *ModelConfigURDF.
func loadURDF(t *testing.T, path string) *ModelConfigURDF {
	t.Helper()

	xmlData, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	urdf := &ModelConfigURDF{}
	err = xml.Unmarshal(xmlData, urdf)
	test.That(t, err, test.ShouldBeNil)
	return urdf
}

func TestURDFToDHParamsUR5e(t *testing.T) {
	urdf := loadURDF(t, "testfiles/ur5e-real.urdf")

	got, err := URDFToDHParams(urdf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldHaveLength, len(expectedUR5eDHParams))

	for i, want := range expectedUR5eDHParams {
		test.That(t, got[i].ID, test.ShouldEqual, want.ID)
		test.That(t, got[i].Parent, test.ShouldEqual, want.Parent)
		test.That(t, got[i].D, test.ShouldAlmostEqual, want.D, ur5eTolerance)
		test.That(t, got[i].Theta, test.ShouldAlmostEqual, want.Theta, ur5eTolerance)
		test.That(t, got[i].A, test.ShouldAlmostEqual, want.A, ur5eTolerance)
		test.That(t, got[i].Alpha, test.ShouldAlmostEqual, want.Alpha, ur5eTolerance)
		test.That(t, got[i].Min, test.ShouldAlmostEqual, want.Min, ur5eTolerance)
		test.That(t, got[i].Max, test.ShouldAlmostEqual, want.Max, ur5eTolerance)
	}
}

func TestWalkURDFChainUR5e(t *testing.T) {
	urdf := loadURDF(t, "testfiles/ur5e-real.urdf")
	joints, err := walkURDFChain(urdf)
	test.That(t, err, test.ShouldBeNil)
	names := make([]string, len(joints))
	for i, j := range joints {
		names[i] = j.Name
	}
	// Expected order: base_joint (fixed), base_link-base_link_inertia (fixed),
	// six revolute joints, then wrist_3_link-ft_frame (fixed).
	expected := []string{
		"base_joint",
		"base_link-base_link_inertia",
		"shoulder_pan_joint",
		"shoulder_lift_joint",
		"elbow_joint",
		"wrist_1_joint",
		"wrist_2_joint",
		"wrist_3_joint",
		"wrist_3_link-ft_frame",
	}
	test.That(t, names, test.ShouldResemble, expected)
}

func TestPoseInMeters(t *testing.T) {
	// 0.5m translation along X, no rotation.
	p := &pose{XYZ: "0.5 0 0", RPY: "0 0 0"}
	got := poseInMeters(p)
	pt := got.Point()
	test.That(t, pt.X, test.ShouldAlmostEqual, 0.5, 1e-12)
	test.That(t, pt.Y, test.ShouldAlmostEqual, 0, 1e-12)
	test.That(t, pt.Z, test.ShouldAlmostEqual, 0, 1e-12)
}

func TestJointAxesAtRestUR5e(t *testing.T) {
	urdf := loadURDF(t, "testfiles/ur5e-real.urdf")
	joints, err := walkURDFChain(urdf)
	test.That(t, err, test.ShouldBeNil)

	axes, origins, endPose, err := jointAxesAtRest(joints)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, axes, test.ShouldHaveLength, 6)
	test.That(t, origins, test.ShouldHaveLength, 6)

	// Joint 1 (shoulder_pan) axis: world Z (two cancelling pi rotations leave orientation identity).
	// Origin is recorded AFTER composing j.Origin, so it's (0, 0, 0.1625) -- the joint frame's position
	// in world. Any point on the joint's axis is valid for DH input; this convention keeps origin and
	// axis both expressed in the joint's own frame.
	test.That(t, axes[0].X, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, axes[0].Y, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, axes[0].Z, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, origins[0].X, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, origins[0].Y, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, origins[0].Z, test.ShouldAlmostEqual, 0.1625, 1e-9)

	// Joint 2 (shoulder_lift) origin: (0, 0, 0.1625); axis in world: (0, -1, 0) after Rx(pi/2).
	test.That(t, origins[1].X, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, origins[1].Y, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, origins[1].Z, test.ShouldAlmostEqual, 0.1625, 1e-9)
	test.That(t, axes[1].X, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, axes[1].Y, test.ShouldAlmostEqual, -1, 1e-9)
	test.That(t, axes[1].Z, test.ShouldAlmostEqual, 0, 1e-9)

	// End-effector pose (ft_frame): composed through chain.
	test.That(t, endPose, test.ShouldNotBeNil)
}

func TestWalkURDFChainBranching(t *testing.T) {
	// Synthetic URDF with one link having two child joints.
	xmlStr := `<?xml version="1.0"?>
<robot name="branch">
  <link name="world"/>
  <link name="a"/>
  <link name="b"/>
  <link name="c"/>
  <joint name="j1" type="fixed">
    <parent link="world"/><child link="a"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
  </joint>
  <joint name="j2" type="fixed">
    <parent link="a"/><child link="b"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
  </joint>
  <joint name="j3" type="fixed">
    <parent link="a"/><child link="c"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
  </joint>
</robot>`
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal([]byte(xmlStr), urdf)
	test.That(t, err, test.ShouldBeNil)

	_, err = walkURDFChain(urdf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "branching")
}

func TestCommonNormalPerpendicular(t *testing.T) {
	// Z-axis through origin + Y-axis through (0, 0, 1): perpendicular intersecting lines.
	z0 := r3.Vector{X: 0, Y: 0, Z: 1}
	p0 := r3.Vector{X: 0, Y: 0, Z: 0}
	z1 := r3.Vector{X: 0, Y: 1, Z: 0}
	p1 := r3.Vector{X: 0, Y: 0, Z: 1}

	xDir, foot0, foot1, parallel := commonNormal(z0, p0, z1, p1)
	test.That(t, parallel, test.ShouldBeFalse)
	// Common normal direction: z0 x z1 = (0,0,1) x (0,1,0) = (-1,0,0)
	test.That(t, xDir.X, test.ShouldAlmostEqual, -1, 1e-9)
	test.That(t, xDir.Y, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, xDir.Z, test.ShouldAlmostEqual, 0, 1e-9)
	// Lines intersect at (0, 0, 1); both feet should be there.
	test.That(t, foot0.Z, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, foot1.Z, test.ShouldAlmostEqual, 1, 1e-9)
}

func TestCommonNormalParallel(t *testing.T) {
	// Two parallel Z-axes separated by 0.5 in X.
	z0 := r3.Vector{X: 0, Y: 0, Z: 1}
	p0 := r3.Vector{X: 0, Y: 0, Z: 0}
	z1 := r3.Vector{X: 0, Y: 0, Z: 1}
	p1 := r3.Vector{X: 0.5, Y: 0, Z: 2} // parallel, offset in X and Z

	xDir, _, _, parallel := commonNormal(z0, p0, z1, p1)
	test.That(t, parallel, test.ShouldBeTrue)
	// Perpendicular direction from line1 to line2 projected off Z: (0.5, 0, 2) perpendicular component
	// wrt z0 is (0.5, 0, 0), normalized is (1, 0, 0).
	test.That(t, xDir.X, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, xDir.Y, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, xDir.Z, test.ShouldAlmostEqual, 0, 1e-9)
}

func TestBuildDHFramesUR5e(t *testing.T) {
	urdf := loadURDF(t, "testfiles/ur5e-real.urdf")
	joints, err := walkURDFChain(urdf)
	test.That(t, err, test.ShouldBeNil)
	axes, origins, endPose, err := jointAxesAtRest(joints)
	test.That(t, err, test.ShouldBeNil)

	zs, xs, pts, err := buildDHFrames(axes, origins, endPose)
	test.That(t, err, test.ShouldBeNil)

	// 7 frames: frame 0 (base) plus 6 frame-per-joint.
	test.That(t, zs, test.ShouldHaveLength, 7)
	test.That(t, xs, test.ShouldHaveLength, 7)
	test.That(t, pts, test.ShouldHaveLength, 7)

	// Frame 0: Z along world Z, X along world X, origin at world origin.
	test.That(t, zs[0].Z, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, xs[0].X, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, pts[0].X, test.ShouldAlmostEqual, 0, 1e-9)

	// Frame 1: Z along -Y (joint 2 axis), X along world X (common normal of Z0 and Z1).
	test.That(t, zs[1].Y, test.ShouldAlmostEqual, -1, 1e-9)
	test.That(t, xs[1].X, test.ShouldAlmostEqual, 1, 1e-9)
	test.That(t, pts[1].Z, test.ShouldAlmostEqual, 0.1625, 1e-9)
}

func TestExtractDHRowUR5eRow1(t *testing.T) {
	// Frame 0: origin (0,0,0), Z=(0,0,1), X=(1,0,0).
	// Frame 1: origin (0,0,0.1625), Z=(0,-1,0), X=(1,0,0).
	// Expected DH row: d=0.1625, a=0, alpha=pi/2, theta=0.
	zPrev := r3.Vector{X: 0, Y: 0, Z: 1}
	xPrev := r3.Vector{X: 1, Y: 0, Z: 0}
	pPrev := r3.Vector{X: 0, Y: 0, Z: 0}
	zCurr := r3.Vector{X: 0, Y: -1, Z: 0}
	xCurr := r3.Vector{X: 1, Y: 0, Z: 0}
	pCurr := r3.Vector{X: 0, Y: 0, Z: 0.1625}

	d, theta, a, alpha := extractDHRow(zPrev, xPrev, pPrev, zCurr, xCurr, pCurr)
	test.That(t, d, test.ShouldAlmostEqual, 0.1625, 1e-9)
	test.That(t, theta, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, a, test.ShouldAlmostEqual, 0, 1e-9)
	test.That(t, alpha, test.ShouldAlmostEqual, math.Pi/2, 1e-9)
}

func TestValidateEndEffectorDHCompatible(t *testing.T) {
	// Z_{N-1} = Z, x_end = X, p_end and origin_{N-1} on same Z axis -> compatible.
	err := validateEndEffectorDH(
		r3.Vector{X: 0, Y: 0, Z: 1}, // zPrev
		r3.Vector{X: 1, Y: 0, Z: 0}, // xEnd
		r3.Vector{X: 0, Y: 0, Z: 0}, // originPrev
		r3.Vector{X: 0, Y: 0, Z: 1}, // pEnd
	)
	test.That(t, err, test.ShouldBeNil)
}

func TestValidateEndEffectorDHXNotPerpendicular(t *testing.T) {
	// x_end has a component along Z_{N-1}: should error.
	err := validateEndEffectorDH(
		r3.Vector{X: 0, Y: 0, Z: 1},
		r3.Vector{X: 1, Y: 0, Z: 0.5}, // not perpendicular to Z
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: 1},
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not perpendicular")
}

func TestValidateEndEffectorDHOriginOutOfPlane(t *testing.T) {
	// p_end - originPrev has a component along Z x X: should error.
	err := validateEndEffectorDH(
		r3.Vector{X: 0, Y: 0, Z: 1},
		r3.Vector{X: 1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 0.5, Y: 0.5, Z: 1}, // Y component -> out of (Z, X) plane
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "out of DH plane")
}

func TestURDFToDHParamsNoRevoluteJoints(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<robot name="fixed_only">
  <link name="world"/>
  <link name="a"/>
  <joint name="j1" type="fixed">
    <parent link="world"/><child link="a"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
  </joint>
</robot>`
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal([]byte(xmlStr), urdf)
	test.That(t, err, test.ShouldBeNil)

	_, err = URDFToDHParams(urdf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no revolute joints")
}

func TestURDFToDHParamsUnsupportedPrismatic(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<robot name="has_prismatic">
  <link name="world"/>
  <link name="a"/>
  <joint name="slide" type="prismatic">
    <parent link="world"/><child link="a"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
    <axis xyz="1 0 0"/>
    <limit effort="0" lower="-1" upper="1" velocity="0"/>
  </joint>
</robot>`
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal([]byte(xmlStr), urdf)
	test.That(t, err, test.ShouldBeNil)

	_, err = URDFToDHParams(urdf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported type")
}

func TestURDFToDHParamsBranching(t *testing.T) {
	xmlStr := `<?xml version="1.0"?>
<robot name="branching">
  <link name="world"/>
  <link name="a"/>
  <link name="b"/>
  <link name="c"/>
  <joint name="j1" type="revolute">
    <parent link="world"/><child link="a"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
    <axis xyz="0 0 1"/>
    <limit effort="0" lower="-1" upper="1" velocity="0"/>
  </joint>
  <joint name="j2" type="revolute">
    <parent link="a"/><child link="b"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
    <axis xyz="0 0 1"/>
    <limit effort="0" lower="-1" upper="1" velocity="0"/>
  </joint>
  <joint name="j3" type="revolute">
    <parent link="a"/><child link="c"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
    <axis xyz="0 0 1"/>
    <limit effort="0" lower="-1" upper="1" velocity="0"/>
  </joint>
</robot>`
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal([]byte(xmlStr), urdf)
	test.That(t, err, test.ShouldBeNil)

	_, err = URDFToDHParams(urdf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "branching")
}

func TestURDFToDHParamsNonDHCompatibleEnd(t *testing.T) {
	// One revolute joint about Z, then a fixed post-chain with a Y rotation --
	// the end frame's X axis will not be perpendicular to Z, failing validation.
	xmlStr := `<?xml version="1.0"?>
<robot name="bad_end">
  <link name="world"/>
  <link name="a"/>
  <link name="b"/>
  <joint name="j1" type="revolute">
    <parent link="world"/><child link="a"/>
    <origin rpy="0 0 0" xyz="0 0 0"/>
    <axis xyz="0 0 1"/>
    <limit effort="0" lower="-1" upper="1" velocity="0"/>
  </joint>
  <joint name="tilt" type="fixed">
    <parent link="a"/><child link="b"/>
    <origin rpy="0 0.5 0" xyz="0 0 0"/>
  </joint>
</robot>`
	urdf := &ModelConfigURDF{}
	err := xml.Unmarshal([]byte(xmlStr), urdf)
	test.That(t, err, test.ShouldBeNil)

	_, err = URDFToDHParams(urdf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not perpendicular")
}

// dhForwardKinematics computes the end-effector pose by composing each DH row.
// For row i: T_i = Rz(theta_i) * Tz(d_i) * Tx(a_i) * Rx(alpha_i).
// Consumes URDFToDHParams's SI output (meters, radians), matching the units of
// urdfEndPoseAtRest's pose so the two are directly comparable.
func dhForwardKinematics(params []DHParamConfig) spatialmath.Pose {
	cumulative := spatialmath.NewZeroPose()
	for _, p := range params {
		cumulative = spatialmath.Compose(cumulative, spatialmath.NewPose(
			r3.Vector{X: 0, Y: 0, Z: 0},
			&spatialmath.EulerAngles{Yaw: p.Theta},
		))
		cumulative = spatialmath.Compose(cumulative, spatialmath.NewPoseFromPoint(r3.Vector{Z: p.D}))
		cumulative = spatialmath.Compose(cumulative, spatialmath.NewPoseFromPoint(r3.Vector{X: p.A}))
		cumulative = spatialmath.Compose(cumulative, spatialmath.NewPose(
			r3.Vector{X: 0, Y: 0, Z: 0},
			&spatialmath.EulerAngles{Roll: p.Alpha},
		))
	}
	return cumulative
}

// urdfEndPoseAtRest computes the URDF's end-effector pose at zero config
// by direct composition -- independent of the DH machinery.
func urdfEndPoseAtRest(t *testing.T, urdf *ModelConfigURDF) spatialmath.Pose {
	t.Helper()
	chain, err := walkURDFChain(urdf)
	test.That(t, err, test.ShouldBeNil)
	cumulative := spatialmath.NewZeroPose()
	for _, j := range chain {
		cumulative = spatialmath.Compose(cumulative, poseInMeters(j.Origin))
	}
	return cumulative
}

func TestURDFToDHParamsUR5e_FKRoundTrip(t *testing.T) {
	urdf := loadURDF(t, "testfiles/ur5e-real.urdf")
	params, err := URDFToDHParams(urdf)
	test.That(t, err, test.ShouldBeNil)

	dhPose := dhForwardKinematics(params)
	urdfPose := urdfEndPoseAtRest(t, urdf)

	dhPt := dhPose.Point()
	urdfPt := urdfPose.Point()
	test.That(t, dhPt.X, test.ShouldAlmostEqual, urdfPt.X, 1e-6)
	test.That(t, dhPt.Y, test.ShouldAlmostEqual, urdfPt.Y, 1e-6)
	test.That(t, dhPt.Z, test.ShouldAlmostEqual, urdfPt.Z, 1e-6)

	// Orientation: compare via OrientationVectorRadians for a rotation-space check.
	dhOV := dhPose.Orientation().OrientationVectorRadians()
	urdfOV := urdfPose.Orientation().OrientationVectorRadians()
	test.That(t, dhOV.OX, test.ShouldAlmostEqual, urdfOV.OX, 1e-6)
	test.That(t, dhOV.OY, test.ShouldAlmostEqual, urdfOV.OY, 1e-6)
	test.That(t, dhOV.OZ, test.ShouldAlmostEqual, urdfOV.OZ, 1e-6)
	test.That(t, dhOV.Theta, test.ShouldAlmostEqual, urdfOV.Theta, 1e-6)
}

func TestURDFToDHParamsGP12_FKRoundTrip(t *testing.T) {
	urdf := loadURDF(t, "testfiles/gp12.urdf")
	params, err := URDFToDHParams(urdf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, params, test.ShouldHaveLength, 6)

	dhPose := dhForwardKinematics(params)
	urdfPose := urdfEndPoseAtRest(t, urdf)

	dhPt := dhPose.Point()
	urdfPt := urdfPose.Point()
	test.That(t, dhPt.X, test.ShouldAlmostEqual, urdfPt.X, 1e-6)
	test.That(t, dhPt.Y, test.ShouldAlmostEqual, urdfPt.Y, 1e-6)
	test.That(t, dhPt.Z, test.ShouldAlmostEqual, urdfPt.Z, 1e-6)

	dhOV := dhPose.Orientation().OrientationVectorRadians()
	urdfOV := urdfPose.Orientation().OrientationVectorRadians()
	test.That(t, dhOV.OX, test.ShouldAlmostEqual, urdfOV.OX, 1e-6)
	test.That(t, dhOV.OY, test.ShouldAlmostEqual, urdfOV.OY, 1e-6)
	test.That(t, dhOV.OZ, test.ShouldAlmostEqual, urdfOV.OZ, 1e-6)
	test.That(t, dhOV.Theta, test.ShouldAlmostEqual, urdfOV.Theta, 1e-6)
}
