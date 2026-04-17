package referenceframe

import (
	"math"
	"testing"

	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestFlattenSerialModel(t *testing.T) {
	// Load the fake arm model (1-DoF serial chain: base_link → shoulder_pan_joint)
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Build a frame system part
	lif := NewLinkInFrame(World, spatial.NewPoseFromPoint(spatial.NewZeroPose().Point()), "arm1", nil)
	parts := []*FrameSystemPart{
		{FrameConfig: lif, ModelFrame: model},
	}

	fs, err := NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)

	// Verify the SimpleModel is in the FS with its full DoF
	arm1Frame := fs.Frame("arm1")
	test.That(t, arm1Frame, test.ShouldNotBeNil)
	test.That(t, len(arm1Frame.DoF()), test.ShouldEqual, 1) // original SimpleModel DoF

	// Verify flattened internal frames are accessible
	test.That(t, fs.Frame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.Frame("arm1:shoulder_pan_joint"), test.ShouldNotBeNil)
	test.That(t, fs.Frame("arm1_origin"), test.ShouldNotBeNil)

	// Internal frames are hidden from FrameNames()
	frameNames := fs.FrameNames()
	for _, name := range frameNames {
		test.That(t, name, test.ShouldNotEqual, "arm1:base_link")
		test.That(t, name, test.ShouldNotEqual, "arm1:shoulder_pan_joint")
	}

	// NewZeroInputs returns component-level entry
	zeroInputs := NewZeroInputs(fs)
	armInputs, ok := zeroInputs["arm1"]
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(armInputs), test.ShouldEqual, 1)

	// No per-frame entries in NewZeroInputs
	_, hasPerFrame := zeroInputs["arm1:shoulder_pan_joint"]
	test.That(t, hasPerFrame, test.ShouldBeFalse)
}

func TestFlattenTransformEquivalence(t *testing.T) {
	// Load a model
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Test at several input configurations
	testInputs := [][]Input{
		{0},
		{math.Pi / 4},
		{-math.Pi / 2},
		{math.Pi},
	}

	for _, inputs := range testInputs {
		// Compute the expected pose using the model directly
		// The model's Transform returns pose relative to its internal world
		expectedPose, err := model.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)

		// Build a flattened frame system
		lif := NewLinkInFrame(World, spatial.NewZeroPose(), "arm1", nil)
		parts := []*FrameSystemPart{
			{FrameConfig: lif, ModelFrame: model},
		}
		fs, err := NewFrameSystem("test", parts, nil)
		test.That(t, err, test.ShouldBeNil)

		// Set up component-level inputs for the FS
		li := NewZeroLinearInputs(fs)
		li.Put("arm1", inputs)

		// Transform from arm1 to world — uses SimpleModel's Transform
		pif := NewPoseInFrame("arm1", spatial.NewZeroPose())
		result, err := fs.Transform(li, pif, World)
		test.That(t, err, test.ShouldBeNil)

		resultPose := result.(*PoseInFrame).Pose()
		test.That(t, spatial.PoseAlmostCoincident(resultPose, expectedPose), test.ShouldBeTrue)
	}
}

func TestFlattenBranchingMimicModel(t *testing.T) {
	// Load the mimic gripper model (branching: base → left_joint, right_joint mimics left)
	model, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 1) // only left_joint is controllable

	// Build flattened FS
	lif := NewLinkInFrame(World, spatial.NewZeroPose(), "gripper1", nil)
	parts := []*FrameSystemPart{
		{FrameConfig: lif, ModelFrame: model},
	}
	fs, err := NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)

	// Mimic frame is a real joint (DoF=1) — mimic info lives on the FS, not the frame
	rightJoint := fs.Frame("gripper1:right_joint")
	test.That(t, rightJoint, test.ShouldNotBeNil)
	test.That(t, len(rightJoint.DoF()), test.ShouldEqual, 1)

	// Verify left_joint has DoF
	leftJoint := fs.Frame("gripper1:left_joint")
	test.That(t, leftJoint, test.ShouldNotBeNil)
	test.That(t, len(leftJoint.DoF()), test.ShouldEqual, 1)

	// The gripper SimpleModel is in the FS with its full DoF
	gripperFrame := fs.Frame("gripper1")
	test.That(t, gripperFrame, test.ShouldNotBeNil)
	test.That(t, len(gripperFrame.DoF()), test.ShouldEqual, 1)
}

func TestFlattenIntermediateParenting(t *testing.T) {
	// Load the fake arm model
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Build a frame system with the arm and an extra frame parented to an intermediate link
	lif := NewLinkInFrame(World, spatial.NewZeroPose(), "arm1", nil)
	cameraOffset := spatial.NewPoseFromPoint(spatial.NewZeroPose().Point())
	cameraLif := NewLinkInFrame("arm1:base_link", cameraOffset, "camera", nil)

	parts := []*FrameSystemPart{
		{FrameConfig: lif, ModelFrame: model},
	}
	fs, err := NewFrameSystem("test", parts, []*LinkInFrame{cameraLif})
	test.That(t, err, test.ShouldBeNil)

	// The camera should be parented to the arm component (Parent masks internal frames)
	test.That(t, fs.Frame("camera"), test.ShouldNotBeNil)
	cameraParent, err := fs.Parent(fs.Frame("camera"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cameraParent.Name(), test.ShouldEqual, "camera_origin")
	// camera_origin's raw parent is arm1:base_link, but Parent() returns the SimpleModel
	originParent, err := fs.Parent(fs.Frame("camera_origin"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, originParent.Name(), test.ShouldEqual, "arm1")

	// Verify the camera's world transform depends on the arm's base link
	li := NewZeroLinearInputs(fs)
	pif := NewPoseInFrame("camera", spatial.NewZeroPose())
	result, err := fs.Transform(li, pif, World)
	test.That(t, err, test.ShouldBeNil)

	// The camera should be at the base_link position.
	// base_link is a static frame in the arm at (500, 0, 300) from the arm origin.
	resultPose := result.(*PoseInFrame).Pose()
	baseLinkPose, err := fs.Frame("arm1:base_link").Transform([]Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(resultPose, baseLinkPose), test.ShouldBeTrue)
}

func TestFlattenComponentLevelTransform(t *testing.T) {
	// Verify that Transform works with component-level LinearInputs (resolveFrameInputs path)
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	lif := NewLinkInFrame(World, spatial.NewZeroPose(), "arm1", nil)
	cameraLif := NewLinkInFrame("arm1:base_link", spatial.NewZeroPose(), "camera", nil)

	parts := []*FrameSystemPart{
		{FrameConfig: lif, ModelFrame: model},
	}
	fs, err := NewFrameSystem("test", parts, []*LinkInFrame{cameraLif})
	test.That(t, err, test.ShouldBeNil)

	// Create component-level LinearInputs (what external code would produce)
	li := NewZeroLinearInputs(fs)
	li.Put("arm1", []Input{math.Pi / 4})

	// Transform from camera to world (goes through individual flattened frames)
	// This exercises the resolveFrameInputs path for arm1:base_link
	pif := NewPoseInFrame("camera", spatial.NewZeroPose())
	result, err := fs.Transform(li, pif, World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldNotBeNil)

	// Also verify Transform from arm1 to world (goes through SimpleModel)
	pifArm := NewPoseInFrame("arm1", spatial.NewZeroPose())
	resultArm, err := fs.Transform(li, pifArm, World)
	test.That(t, err, test.ShouldBeNil)

	// arm1 result should match the model's FK
	expectedPose, err := model.Transform([]Input{math.Pi / 4})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(resultArm.(*PoseInFrame).Pose(), expectedPose), test.ShouldBeTrue)
}
