package referenceframe

import (
	"encoding/json"
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
	test.That(t, fs.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("arm1:shoulder_pan_joint"), test.ShouldNotBeNil)
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
	rightJoint := fs.lookupFrame("gripper1:right_joint")
	test.That(t, rightJoint, test.ShouldNotBeNil)
	test.That(t, len(rightJoint.DoF()), test.ShouldEqual, 1)

	// Verify left_joint has DoF
	leftJoint := fs.lookupFrame("gripper1:left_joint")
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
	baseLinkPose, err := fs.lookupFrame("arm1:base_link").Transform([]Input{})
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

// flattenFSFixture builds an FS containing a flattened arm with a camera
// parented to an internal joint of that arm. Used as a fixture for round-trip,
// clone, subset, and merge tests that need to exercise both flattening
// metadata and externals-attached-to-internals.
func flattenFSFixture(t *testing.T) *FrameSystem {
	t.Helper()
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)

	armLif := NewLinkInFrame(World, spatial.NewZeroPose(), "arm1", nil)
	cameraLif := NewLinkInFrame("arm1:base_link", spatial.NewPoseFromPoint(spatial.NewZeroPose().Point()), "camera", nil)

	parts := []*FrameSystemPart{{FrameConfig: armLif, ModelFrame: model}}
	fs, err := NewFrameSystem("flattenFix", parts, []*LinkInFrame{cameraLif})
	test.That(t, err, test.ShouldBeNil)
	return fs
}

func TestFlattenedRoundTripJSON(t *testing.T) {
	fs := flattenFSFixture(t)

	data, err := json.Marshal(fs)
	test.That(t, err, test.ShouldBeNil)

	var fs2 FrameSystem
	test.That(t, json.Unmarshal(data, &fs2), test.ShouldBeNil)

	eq, err := frameSystemsAlmostEqual(fs, &fs2, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	// Internals and the externally-attached camera both survive.
	test.That(t, fs2.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs2.Frame("camera"), test.ShouldNotBeNil)
	test.That(t, fs2.parents["camera_origin"], test.ShouldEqual, "arm1:base_link")
}

func TestFlattenedSubsetClonesEquivalent(t *testing.T) {
	fs := flattenFSFixture(t)

	subset, err := fs.FrameSystemSubset(fs.Frame("arm1"))
	test.That(t, err, test.ShouldBeNil)

	cloned, err := cloneFrameSystem(subset)
	test.That(t, err, test.ShouldBeNil)

	eq, err := frameSystemsAlmostEqual(subset, cloned, defaultFloatPrecision)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, eq, test.ShouldBeTrue)

	// Both copies preserve flattened internals and metadata.
	for _, fsCopy := range []*FrameSystem{subset, cloned} {
		test.That(t, fsCopy.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
		test.That(t, fsCopy.flattened["arm1"], test.ShouldNotBeNil)
		test.That(t, fsCopy.flattened["arm1"].model, test.ShouldNotBeNil)
		test.That(t, fsCopy.flattened["arm1"].schema, test.ShouldNotBeNil)
		test.That(t, fsCopy.internalToComponent["arm1:base_link"], test.ShouldEqual, "arm1")
	}
}

func TestFlattenedMergePreservesExternalOnInternal(t *testing.T) {
	// Merging an FS that has an external attached to an internal joint
	// must reproduce that attachment in the destination.
	dest := NewEmptyFrameSystem("dest")
	src := flattenFSFixture(t)

	test.That(t, dest.MergeFrameSystem(src, dest.World()), test.ShouldBeNil)
	test.That(t, dest.Frame("arm1"), test.ShouldNotBeNil)
	test.That(t, dest.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, dest.Frame("camera"), test.ShouldNotBeNil)
	test.That(t, dest.parents["camera_origin"], test.ShouldEqual, "arm1:base_link")
	test.That(t, dest.internalToComponent["arm1:base_link"], test.ShouldEqual, "arm1")
}

// TestReplaceFlattenedFrameSameShape covers the common ReplaceFrame use case:
// updating an arm's limits or other attributes by swapping in a new SimpleModel
// with an identical internal layout. Externals attached to a particular
// internal joint must continue to work against the new model's matching joint.
func TestReplaceFlattenedFrameSameShape(t *testing.T) {
	fs := flattenFSFixture(t)

	// Reload the same model so the replacement has identical internal names.
	newModel, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "")
	test.That(t, err, test.ShouldBeNil)
	newArm := NewNamedFrame(newModel, "arm1")

	test.That(t, fs.ReplaceFrame(newArm), test.ShouldBeNil)

	// The component is the new instance, internals are reinstalled, and the
	// camera (parented to arm1:base_link) still resolves through the new
	// model.
	test.That(t, fs.Frame("arm1"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.parents["camera_origin"], test.ShouldEqual, "arm1:base_link")
	test.That(t, fs.internalToComponent["arm1:base_link"], test.ShouldEqual, "arm1")

	li := NewZeroLinearInputs(fs)
	pif := NewPoseInFrame("camera", spatial.NewZeroPose())
	_, err = fs.Transform(li, pif, World)
	test.That(t, err, test.ShouldBeNil)
}

// TestRemoveFlattenedFrameCascadesToInternalAttachments covers RemoveFrame's
// cascading-delete semantics in the flattened-component case: externals
// attached to a component's internal joints must be removed along with the
// component, just like direct children are.
func TestRemoveFlattenedFrameCascadesToInternalAttachments(t *testing.T) {
	fs := flattenFSFixture(t)

	fs.RemoveFrame(fs.Frame("arm1"))

	// The arm and all its internals are gone.
	test.That(t, fs.Frame("arm1"), test.ShouldBeNil)
	test.That(t, fs.lookupFrame("arm1:base_link"), test.ShouldBeNil)
	// The camera (which was attached to arm1:base_link) must be gone too,
	// otherwise it dangles with a stale parent pointer.
	test.That(t, fs.Frame("camera"), test.ShouldBeNil)
	test.That(t, fs.Frame("camera_origin"), test.ShouldBeNil)
	_, hasStaleParent := fs.parents["camera_origin"]
	test.That(t, hasStaleParent, test.ShouldBeFalse)
}

// TestReplaceFlattenedFrameOrphansExternals covers the case where the
// replacement would leave externals attached to internals dangling — either
// because the replacement is a non-SimpleModel or because its internal layout
// differs. ReplaceFrame must refuse and leave the FS unchanged.
func TestReplaceFlattenedFrameOrphansExternals(t *testing.T) {
	fs := flattenFSFixture(t)
	originalArm := fs.Frame("arm1")

	// Replacement is a plain static frame with the same component name; it has
	// no internals, so the camera (parented to arm1:base_link) would be
	// orphaned if the swap were allowed to proceed.
	staticArm := NewZeroStaticFrame("arm1")

	err := fs.ReplaceFrame(staticArm)
	test.That(t, err, test.ShouldNotBeNil)

	// FS must be unchanged: original arm is still present, internals are still
	// installed, and the camera still resolves to world.
	test.That(t, fs.Frame("arm1"), test.ShouldEqual, originalArm)
	test.That(t, fs.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.parents["camera_origin"], test.ShouldEqual, "arm1:base_link")
	test.That(t, fs.internalToComponent["arm1:base_link"], test.ShouldEqual, "arm1")

	li := NewZeroLinearInputs(fs)
	pif := NewPoseInFrame("camera", spatial.NewZeroPose())
	_, err = fs.Transform(li, pif, World)
	test.That(t, err, test.ShouldBeNil)
}

// TestAddFrameAutoFlattensSimpleModel verifies that calling AddFrame directly
// with a SimpleModel triggers internal frame flattening — the model's
// internal frames become accessible under the "<componentName>:<internalName>"
// convention without going through the NewFrameSystem path.
func TestAddFrameAutoFlattensSimpleModel(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "arm1")
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	test.That(t, fs.Frame("arm1"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("arm1:shoulder_pan_joint"), test.ShouldNotBeNil)

	for _, name := range fs.FrameNames() {
		test.That(t, name, test.ShouldNotEqual, "arm1:base_link")
		test.That(t, name, test.ShouldNotEqual, "arm1:shoulder_pan_joint")
	}

	li := NewZeroLinearInputs(fs)
	armInputs := li.Get("arm1")
	test.That(t, armInputs, test.ShouldNotBeNil)
	test.That(t, len(armInputs), test.ShouldEqual, 1)
}

// TestSubsetIncludesFlattenedInternals verifies that a subset rooted at a
// flattened SimpleModel includes its regenerated internals plus externals
// attached to those internals, and excludes unrelated siblings.
func TestSubsetIncludesFlattenedInternals(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "arm1")
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	camera := NewZeroStaticFrame("camera")
	test.That(t, fs.AddFrame(camera, fs.lookupFrame("arm1:base_link")), test.ShouldBeNil)

	sibling := NewZeroStaticFrame("sibling")
	test.That(t, fs.AddFrame(sibling, fs.World()), test.ShouldBeNil)

	sub, err := fs.FrameSystemSubset(fs.Frame("arm1"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sub.Frame("arm1"), test.ShouldNotBeNil)
	test.That(t, sub.lookupFrame("arm1:base_link"), test.ShouldNotBeNil)
	test.That(t, sub.lookupFrame("arm1:shoulder_pan_joint"), test.ShouldNotBeNil)
	test.That(t, sub.Frame("camera"), test.ShouldNotBeNil)
	test.That(t, sub.Frame("sibling"), test.ShouldBeNil)

	parent, err := sub.Parent(sub.Frame("arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parent.Name(), test.ShouldEqual, World)
}

// TestReplaceFrameRebuildsFlattenedInternals verifies that ReplaceFrame on a
// flattened SimpleModel tears down the old namespaced internals and rebuilds
// the replacement model's internals under the same component name.
func TestReplaceFrameRebuildsFlattenedInternals(t *testing.T) {
	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/fake.json"), "device")
	test.That(t, err, test.ShouldBeNil)
	gripper, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "device")
	test.That(t, err, test.ShouldBeNil)

	fs := NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(arm, fs.World()), test.ShouldBeNil)
	test.That(t, fs.lookupFrame("device:base_link"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("device:shoulder_pan_joint"), test.ShouldNotBeNil)

	test.That(t, fs.ReplaceFrame(gripper), test.ShouldBeNil)

	test.That(t, fs.lookupFrame("device:base_link"), test.ShouldBeNil)
	test.That(t, fs.lookupFrame("device:shoulder_pan_joint"), test.ShouldBeNil)

	test.That(t, fs.lookupFrame("device:base"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("device:left_joint"), test.ShouldNotBeNil)
	test.That(t, fs.lookupFrame("device:right_joint"), test.ShouldNotBeNil)
}

// TestSharesRigidMotionAcrossInternalJoints verifies that two externals
// parented to different moving internal joints of the same flattened component
// correctly report NOT sharing rigid motion. The walk must use raw parents so
// flattened internals are surfaced rather than masked behind the SimpleModel.
func TestSharesRigidMotionAcrossInternalJoints(t *testing.T) {
	model, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)

	gripperLif := NewLinkInFrame(World, spatial.NewZeroPose(), "gripper1", nil)
	leftAttachLif := NewLinkInFrame("gripper1:left_joint", spatial.NewZeroPose(), "left_attach", nil)
	rightAttachLif := NewLinkInFrame("gripper1:right_joint", spatial.NewZeroPose(), "right_attach", nil)
	leftAttach2Lif := NewLinkInFrame("gripper1:left_joint", spatial.NewZeroPose(), "left_attach2", nil)

	parts := []*FrameSystemPart{{FrameConfig: gripperLif, ModelFrame: model}}
	fs, err := NewFrameSystem("test", parts, []*LinkInFrame{leftAttachLif, rightAttachLif, leftAttach2Lif})
	test.That(t, err, test.ShouldBeNil)

	leftAttach := fs.Frame("left_attach")
	rightAttach := fs.Frame("right_attach")
	leftAttach2 := fs.Frame("left_attach2")
	test.That(t, leftAttach, test.ShouldNotBeNil)
	test.That(t, rightAttach, test.ShouldNotBeNil)
	test.That(t, leftAttach2, test.ShouldNotBeNil)

	// Externals on different internal joints of the same component must NOT share rigid motion.
	test.That(t, fs.SharesRigidMotion(leftAttach, rightAttach), test.ShouldBeFalse)

	// Externals on the same internal joint DO share rigid motion.
	test.That(t, fs.SharesRigidMotion(leftAttach, leftAttach2), test.ShouldBeTrue)
}
