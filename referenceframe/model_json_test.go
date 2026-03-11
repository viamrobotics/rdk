package referenceframe

import (
	"encoding/json"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// TestGripperModelJSON loads a branching gripper model from JSON and verifies that:
//   - the model parses correctly with output_frames pointing to the TCP
//   - Transform() returns the TCP position (midpoint between the fingers, at the tip)
//   - Geometries() correctly places the finger box geometries in world space
//
// Gripper tree:
//
//	world -> base(static) -> left_joint(prismatic, +Y)  -> left_finger(static, box)
//	                      -> right_joint(prismatic, −Y) -> right_finger(static, box)
//	                      -> tcp(static, no geometry)   [output frame]
//
// BFS/schema order: base, left_joint(off=0), right_joint(off=1), tcp, left_finger, right_finger
// inputs: [left_joint_mm, right_joint_mm]
func TestGripperModelJSON(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.Name(), test.ShouldEqual, "test_gripper")
	test.That(t, len(model.DoF()), test.ShouldEqual, 2)

	// inputs: left_joint opens to +25 mm in Y, right_joint opens to +25 mm in −Y
	leftMM, rightMM := 25.0, 25.0
	inputs := []Input{leftMM, rightMM}

	// TCP is a static link at (0,0,30) from base; it does not move with the joints.
	tcpPose, err := model.Transform(inputs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.R3VectorAlmostEqual(tcpPose.Point(), r3.Vector{X: 0, Y: 0, Z: 30}, defaultFloatPrecision), test.ShouldBeTrue)

	geoms, err := model.Geometries(inputs)
	test.That(t, err, test.ShouldBeNil)

	// Geometry offsets are in the parent frame (left_joint / right_joint) coordinate system.
	// The geometry config specifies translation (0,0,15): center of the 30mm finger body
	// measured from the joint, spanning the full finger length from Z=0 to Z=30.
	const fingerBodyCenterZ = 15.0

	// left_finger: base → left_joint at (0,+25,0) → geometry center at (0,+25,15)
	leftPt := geoms.GeometryByName("test_gripper:left_finger").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(
		leftPt, r3.Vector{X: 0, Y: leftMM, Z: fingerBodyCenterZ}, defaultFloatPrecision,
	), test.ShouldBeTrue)

	// right_finger: base → right_joint at (0,−25,0) → geometry center at (0,−25,15)
	rightPt := geoms.GeometryByName("test_gripper:right_finger").Pose().Point()
	test.That(t, spatial.R3VectorAlmostEqual(
		rightPt, r3.Vector{X: 0, Y: -rightMM, Z: fingerBodyCenterZ}, defaultFloatPrecision,
	), test.ShouldBeTrue)

	// Fingers should be symmetric about Y=0 at the same Z.
	test.That(t, spatial.R3VectorAlmostEqual(
		r3.Vector{X: leftPt.X, Y: -leftPt.Y, Z: leftPt.Z},
		r3.Vector{X: rightPt.X, Y: rightPt.Y, Z: rightPt.Z},
		defaultFloatPrecision,
	), test.ShouldBeTrue)
}

func TestLimitsParsing(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	smodel, ok := model.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	// Set custom limits on the simple model. Such that when we roundtrip through JSON
	// serialization, we can see those values persist.
	smodel.limits[0].Min = 0
	smodel.limits[0].Max = 1

	data, err := smodel.MarshalJSON()
	test.That(t, err, test.ShouldBeNil)

	simpleModelDeserialized := new(SimpleModel)
	err = simpleModelDeserialized.UnmarshalJSON(data)
	test.That(t, err, test.ShouldBeNil)

	// Assert that the min/max member values reflect the above assignments. As well as the result of
	// the interface `DoF` method.
	test.That(t, simpleModelDeserialized.limits[0].Min, test.ShouldEqual, 0)
	test.That(t, simpleModelDeserialized.limits[0].Max, test.ShouldEqual, 1)
	test.That(t, simpleModelDeserialized.DoF()[0], test.ShouldResemble, Limit{0, 1})
}

// Tests that yml files are properly parsed and correctly loaded into the model
// Should not need to actually test the contained rotation/translation values
// since that will be caught by tests to the actual kinematics
// So we'll just check that we read in the right number of joints.
func TestParseJSONFile(t *testing.T) {
	goodFiles := []string{
		"components/arm/fake/kinematics/xarm6.json",
		"components/arm/fake/kinematics/xarm7.json",
		"referenceframe/testfiles/ur5eDH.json",
		"components/arm/fake/kinematics/ur5e.json",
		"components/arm/fake/kinematics/dofbot.json",
	}

	badFiles := []string{
		"referenceframe/testfiles/kinematicsloop.json",
		"referenceframe/testfiles/worldjoint.json",
		"referenceframe/testfiles/worldlink.json",
		"referenceframe/testfiles/worldDH.json",
		"referenceframe/testfiles/missinglink.json",
	}

	badFilesErrors := []error{
		ErrCircularReference,
		NewReservedWordError("link", "world"),
		NewReservedWordError("joint", "world"),
		ErrNeedOneEndEffector, // 0 end effectors
		ErrNeedOneEndEffector, // 2 end effectors
	}

	for _, f := range goodFiles {
		t.Run(f, func(tt *testing.T) {
			model, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldBeNil)

			smodel, ok := model.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)
			data, err := json.Marshal(smodel.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			model2, err := UnmarshalModelJSON(data, "")
			test.That(t, err, test.ShouldBeNil)

			smodel2, ok := model2.(*SimpleModel)
			test.That(t, ok, test.ShouldBeTrue)

			data2, err := json.Marshal(smodel2.modelConfig)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, data, test.ShouldResemble, data2)
		})
	}

	for i, f := range badFiles {
		t.Run(f, func(tt *testing.T) {
			_, err := ParseModelJSONFile(utils.ResolveFile(f), "")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, badFilesErrors[i].Error())
		})
	}
}
