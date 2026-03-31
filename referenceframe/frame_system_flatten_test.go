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

	// Verify flattened structure
	t.Log("Frame names:", fs.FrameNames())
	test.That(t, fs.Frame("arm1"), test.ShouldNotBeNil) // backward-compat alias
	test.That(t, fs.Frame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.Frame("arm1:shoulder_pan_joint"), test.ShouldNotBeNil)
	test.That(t, fs.Frame("arm1_origin"), test.ShouldNotBeNil)

	// backward-compat alias is 0-DoF static
	test.That(t, len(fs.Frame("arm1").DoF()), test.ShouldEqual, 0)

	// Individual joint has DoF
	test.That(t, len(fs.Frame("arm1:shoulder_pan_joint").DoF()), test.ShouldEqual, 1)

	// backward-compat alias is parented to the primary output frame
	parent, err := fs.Parent(fs.Frame("arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parent.Name(), test.ShouldEqual, "arm1:shoulder_pan_joint")

	// FlattenedModel returns the original model
	test.That(t, fs.FlattenedModel("arm1"), test.ShouldNotBeNil)
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

		// Set up inputs for the flattened FS
		li := NewZeroLinearInputs(fs)
		li.Put("arm1:shoulder_pan_joint", inputs)

		// Transform from backward-compat alias to world
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

	// Verify mimic frame is 0-DoF in the outer FS
	rightJoint := fs.Frame("gripper1:right_joint")
	test.That(t, rightJoint, test.ShouldNotBeNil)
	test.That(t, len(rightJoint.DoF()), test.ShouldEqual, 0) // mimic wrapper

	// Verify left_joint has DoF
	leftJoint := fs.Frame("gripper1:left_joint")
	test.That(t, leftJoint, test.ShouldNotBeNil)
	test.That(t, len(leftJoint.DoF()), test.ShouldEqual, 1)

	// Check that the mimic frame wrapper is correctly typed
	_, isMimic := rightJoint.(*mimicFrameWrapper)
	test.That(t, isMimic, test.ShouldBeTrue)
}

func TestFlattenDistributeGatherRoundTrip(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	// Build a flattened FS to get the component schema
	lif := NewLinkInFrame(World, spatial.NewZeroPose(), "arm1", nil)
	parts := []*FrameSystemPart{
		{FrameConfig: lif, ModelFrame: model},
	}
	fs, err := NewFrameSystem("test", parts, nil)
	test.That(t, err, test.ShouldBeNil)

	schema := fs.ComponentSchema("arm1")
	test.That(t, schema, test.ShouldNotBeNil)

	original := []Input{1.5}

	// Distribute via FloatsToInputs
	distributed, err := schema.FloatsToInputs(original)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distributed.Get("arm1:shoulder_pan_joint"), test.ShouldResemble, original)

	// Gather via GatherInputs
	gathered := schema.GatherInputs(distributed)
	test.That(t, gathered, test.ShouldResemble, original)
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

	// The camera should be parented to the intermediate base_link
	test.That(t, fs.Frame("camera"), test.ShouldNotBeNil)
	cameraParent, err := fs.Parent(fs.Frame("camera"))
	test.That(t, err, test.ShouldBeNil)
	// camera_origin → arm1:base_link
	test.That(t, cameraParent.Name(), test.ShouldEqual, "camera_origin")
	originParent, err := fs.Parent(fs.Frame("camera_origin"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, originParent.Name(), test.ShouldEqual, "arm1:base_link")

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
