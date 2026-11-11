package referenceframe

import (
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// composeChain does by hand what Transform does.
func composeChain(t *testing.T, chain []ChainFrame, inputs []Input) spatial.Pose {
	t.Helper()
	pose := spatial.NewZeroPose()
	for _, cf := range chain {
		frameInputs := []Input{}
		switch dof := len(cf.Frame.DoF()); {
		case dof == 0:
		case cf.Mimic != nil:
			test.That(t, cf.Offset, test.ShouldEqual, -1)
			frameInputs = []Input{cf.Mimic.ValueMultiplier*inputs[cf.Mimic.Source] + cf.Mimic.ValueOffset}
		default:
			frameInputs = inputs[cf.Offset : cf.Offset+dof]
		}
		step, err := cf.Frame.Transform(frameInputs)
		test.That(t, err, test.ShouldBeNil)
		pose = spatial.Compose(pose, step)
	}
	return pose
}

func assertChainFrames(t *testing.T, chain []ChainFrame) {
	t.Helper()
	for _, cf := range chain {
		_, nested := cf.Frame.(*SimpleModel)
		test.That(t, nested, test.ShouldBeFalse)
		_, named := cf.Frame.(*namedFrame)
		test.That(t, named, test.ShouldBeFalse)
		switch f := cf.Frame.(type) {
		case RevoluteFrame:
			test.That(t, len(f.DoF()), test.ShouldEqual, 1)
			test.That(t, f.RotationAxis().Norm(), test.ShouldAlmostEqual, 1, 1e-9)
		case PrismaticFrame:
			test.That(t, len(f.DoF()), test.ShouldEqual, 1)
			test.That(t, f.TranslationAxis().Norm(), test.ShouldAlmostEqual, 1, 1e-9)
		default:
			test.That(t, len(f.DoF()), test.ShouldEqual, 0)
		}
	}
}

func assertChainMatchesTransform(t *testing.T, m Model, seed int64) {
	t.Helper()
	chain, err := m.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chain, test.ShouldNotBeEmpty)
	assertChainFrames(t, chain)
	rng := rand.New(rand.NewSource(seed))
	for range 25 {
		inputs := RandomFrameInputs(m, rng)
		want, err := m.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqualEps(composeChain(t, chain, inputs), want, 1e-6), test.ShouldBeTrue)
	}
}

func TestChainMatchesTransformForShippedModels(t *testing.T) {
	armFiles, err := filepath.Glob(filepath.Join(utils.ResolveFile("components/arm/kinematics"), "*.json"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(armFiles), test.ShouldBeGreaterThan, 5)
	files := armFiles
	files = append(files,
		utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"),
		utils.ResolveFile("referenceframe/testfiles/example_gantry.json"),
		utils.ResolveFile("referenceframe/testfiles/test_mimic_serial.json"),
		utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"),
	)
	for i, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			m, err := KinematicModelFromFile(file, "")
			test.That(t, err, test.ShouldBeNil)
			assertChainMatchesTransform(t, m, int64(i))
		})
	}
}

// The gripper's tcp hangs off its base, so neither finger joint is on the model's chain, while a
// chain to a finger carries its joint, the mimic one with a source instead of an offset.
func TestChainOffsetsShowBranchesAndMimics(t *testing.T) {
	gripper, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gripper.DoF()), test.ShouldEqual, 1)

	tcp, err := gripper.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(tcp), test.ShouldResemble, []string{"base", "tcp"})

	left, err := gripper.Chain("left_finger")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(left), test.ShouldResemble, []string{"base", "left_joint", "left_finger"})
	test.That(t, left[1].Offset, test.ShouldEqual, 0)
	test.That(t, left[1].Mimic, test.ShouldBeNil)
	_, prismatic := left[1].Frame.(PrismaticFrame)
	test.That(t, prismatic, test.ShouldBeTrue)

	right, err := gripper.Chain("right_finger")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(right), test.ShouldResemble, []string{"base", "right_joint", "right_finger"})
	test.That(t, right[1].Offset, test.ShouldEqual, -1)
	test.That(t, right[1].Mimic, test.ShouldResemble, &MimicInput{Source: 0, ValueMultiplier: 1})

	open := []Input{0.02}
	leftPose := composeChain(t, left, open)
	rightPose := composeChain(t, right, open)
	test.That(t, leftPose.Point().Norm(), test.ShouldAlmostEqual, rightPose.Point().Norm(), 1e-9)

	serial, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_serial.json"), "")
	test.That(t, err, test.ShouldBeNil)
	chain, err := serial.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(chain), test.ShouldResemble, []string{"link0", "joint1", "link1", "joint2", "link2", "joint3", "link3"})
	test.That(t, chain[1].Offset, test.ShouldEqual, 0)
	test.That(t, chain[3].Offset, test.ShouldEqual, 1)
	test.That(t, chain[5].Mimic, test.ShouldResemble, &MimicInput{Source: 0, ValueMultiplier: -1})
}

func chainNames(chain []ChainFrame) []string {
	names := make([]string, 0, len(chain))
	for _, cf := range chain {
		names = append(names, cf.Frame.Name())
	}
	return names
}

// A model built over a frame system of parts holds each part's model as one frame, and the chain
// expands them with the offsets where the part's inputs sit.
func TestChainExpandsNestedModels(t *testing.T) {
	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	gripper, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)

	mount := spatial.NewPose(r3.Vector{X: 100, Y: 50, Z: 10}, &spatial.OrientationVectorDegrees{OZ: 1, Theta: 90})
	parts := []*FrameSystemPart{
		{FrameConfig: NewLinkInFrame(World, mount, "arm", nil), ModelFrame: arm},
		{FrameConfig: NewLinkInFrame("arm", spatial.NewPoseFromPoint(r3.Vector{Z: 20}), "hand", nil), ModelFrame: gripper},
	}
	probe := NewLinkInFrame("hand", spatial.NewPoseFromPoint(r3.Vector{Z: 30}), "probe", nil)
	fs, err := NewFrameSystem("machine", parts, []*LinkInFrame{probe})
	test.That(t, err, test.ShouldBeNil)
	composed, err := NewModel("composed", fs, "probe")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(composed.DoF()), test.ShouldEqual, 7)

	chain, err := composed.Chain("")
	test.That(t, err, test.ShouldBeNil)
	assertChainFrames(t, chain)
	var revolute, offsets []int
	for i, cf := range chain {
		if _, ok := cf.Frame.(RevoluteFrame); ok {
			revolute = append(revolute, i)
			offsets = append(offsets, cf.Offset)
		}
	}
	test.That(t, len(revolute), test.ShouldEqual, 6)
	test.That(t, offsets, test.ShouldResemble, []int{0, 1, 2, 3, 4, 5})
	test.That(t, chainNames(chain)[0], test.ShouldEqual, "arm_origin")
	test.That(t, chainNames(chain)[len(chain)-1], test.ShouldEqual, "probe")

	assertChainMatchesTransform(t, composed, 7)

	finger, err := composed.Chain("hand:right_finger")
	test.That(t, err, test.ShouldBeNil)
	assertChainFrames(t, finger)
	test.That(t, chainNames(finger)[len(finger)-3:], test.ShouldResemble, []string{"base", "right_joint", "right_finger"})
	test.That(t, finger[len(finger)-2].Mimic, test.ShouldResemble, &MimicInput{Source: 6, ValueMultiplier: 1})
	leftFinger, err := composed.Chain("hand:left_finger")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, leftFinger[len(leftFinger)-2].Offset, test.ShouldEqual, 6)

	// the frame system is the only other forward kinematics for a frame inside a part
	rng := rand.New(rand.NewSource(11))
	for range 10 {
		inputs := RandomFrameInputs(composed, rng)
		li := NewLinearInputs()
		li.Put("arm", inputs[:6])
		li.Put("hand", inputs[6:])
		want, err := fs.Transform(li, NewPoseInFrame("hand:right_finger", spatial.NewZeroPose()), World)
		test.That(t, err, test.ShouldBeNil)
		got := composeChain(t, finger, inputs)
		test.That(t, spatial.PoseAlmostEqualEps(got, want.(*PoseInFrame).Pose(), 1e-6), test.ShouldBeTrue)
	}
}

// A renamed frame hides the axis methods, which is why Chain unwraps it.
func TestAxisInterfaces(t *testing.T) {
	limit := Limit{Min: -1, Max: 1}
	rot, err := NewRotationalFrame("rot", spatial.R4AA{RX: 0, RY: 0, RZ: 2}, limit)
	test.That(t, err, test.ShouldBeNil)
	revolute, ok := rot.(RevoluteFrame)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, revolute.RotationAxis(), test.ShouldResemble, r3.Vector{Z: 1})
	_, ok = rot.(PrismaticFrame)
	test.That(t, ok, test.ShouldBeFalse)

	slide, err := NewTranslationalFrame("slide", r3.Vector{Y: 3}, limit)
	test.That(t, err, test.ShouldBeNil)
	prismatic, ok := slide.(PrismaticFrame)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, prismatic.TranslationAxis(), test.ShouldResemble, r3.Vector{Y: 1})
	_, ok = slide.(RevoluteFrame)
	test.That(t, ok, test.ShouldBeFalse)

	static, err := NewStaticFrame("static", spatial.NewPoseFromPoint(r3.Vector{Z: 5}))
	test.That(t, err, test.ShouldBeNil)
	_, ok = static.(RevoluteFrame)
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = static.(PrismaticFrame)
	test.That(t, ok, test.ShouldBeFalse)

	renamed := NewNamedFrame(rot, "j1")
	_, ok = renamed.(RevoluteFrame)
	test.That(t, ok, test.ShouldBeFalse)
	model, err := NewSerialModel("built", []Frame{renamed, static})
	test.That(t, err, test.ShouldBeNil)
	chain, err := model.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(chain), test.ShouldResemble, []string{"rot", "static"})
	_, ok = chain[0].Frame.(RevoluteFrame)
	test.That(t, ok, test.ShouldBeTrue)
	assertChainMatchesTransform(t, model, 3)
}

func TestChainRefusesAFrameTheModelLacks(t *testing.T) {
	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	_, err = arm.Chain("nope")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "nope")
}

// A gripper without geometry is a model with no frames, and a part with such a model composes
// like any other.
func TestChainOfNothingIsEmpty(t *testing.T) {
	empty := NewSimpleModel("empty")
	chain, err := empty.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chain, test.ShouldBeEmpty)
	pose, err := empty.Transform(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostEqual(pose, spatial.NewZeroPose()), test.ShouldBeTrue)

	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	chain, err = arm.Chain(World)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chain, test.ShouldBeEmpty)

	parts := []*FrameSystemPart{
		{FrameConfig: NewLinkInFrame(World, spatial.NewZeroPose(), "arm", nil), ModelFrame: arm},
		{FrameConfig: NewLinkInFrame("arm", spatial.NewPoseFromPoint(r3.Vector{Z: 20}), "hand", nil), ModelFrame: NewSimpleModel("hand")},
	}
	probe := NewLinkInFrame("hand", spatial.NewPoseFromPoint(r3.Vector{Z: 30}), "probe", nil)
	fs, err := NewFrameSystem("machine", parts, []*LinkInFrame{probe})
	test.That(t, err, test.ShouldBeNil)
	composed, err := NewModel("composed", fs, "probe")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(composed.DoF()), test.ShouldEqual, 6)
	assertChainMatchesTransform(t, composed, 13)
	toHand, err := composed.Chain("hand")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(toHand)[len(toHand)-1], test.ShouldEqual, "hand_origin")
}

// The frame system flattens a part nested in a part under two prefixes.
func TestChainReachesAModelInsideAModelInsideAModel(t *testing.T) {
	gripper, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	inner := NewEmptyFrameSystem("inner")
	wrist, err := NewRotationalFrame("wrist", spatial.R4AA{RY: 1}, Limit{Min: -1, Max: 1})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inner.AddFrame(wrist, inner.World()), test.ShouldBeNil)
	test.That(t, inner.AddFrame(NewNamedFrame(gripper, "grip"), wrist), test.ShouldBeNil)
	tool, err := NewModel("tool", inner, "grip")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(tool.DoF()), test.ShouldEqual, 2)

	outer := NewEmptyFrameSystem("outer")
	test.That(t, outer.AddFrame(NewNamedFrame(tool, "tool"), outer.World()), test.ShouldBeNil)
	probe, err := NewStaticFrame("probe", spatial.NewPoseFromPoint(r3.Vector{Z: 5}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outer.AddFrame(probe, outer.lookupFrame("tool:grip:right_finger")), test.ShouldBeNil)
	model, err := NewModel("outer", outer, "probe")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 2)

	chain, err := model.Chain("")
	test.That(t, err, test.ShouldBeNil)
	assertChainFrames(t, chain)
	test.That(t, chainNames(chain), test.ShouldResemble, []string{"wrist", "base", "right_joint", "right_finger", "probe"})
	test.That(t, chain[0].Offset, test.ShouldEqual, 0)
	test.That(t, chain[2].Mimic, test.ShouldResemble, &MimicInput{Source: 1, ValueMultiplier: 1})
	left, err := model.Chain("tool:grip:left_finger")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(left), test.ShouldResemble, []string{"wrist", "base", "left_joint", "left_finger"})
	test.That(t, left[2].Offset, test.ShouldEqual, 1)

	for _, q := range [][]Input{{0, 0}, {0.5, 0.02}, {-1, 0.01}} {
		want, err := wrist.Transform(q[:1])
		test.That(t, err, test.ShouldBeNil)
		for _, name := range []string{"base", "right_joint", "right_finger"} {
			f := gripper.(*SimpleModel).internalFS.Frame(name)
			step, err := f.Transform(q[1:][:len(f.DoF())])
			test.That(t, err, test.ShouldBeNil)
			want = spatial.Compose(want, step)
		}
		want = spatial.Compose(want, spatial.NewPoseFromPoint(r3.Vector{Z: 5}))
		test.That(t, spatial.PoseAlmostEqualEps(composeChain(t, chain, q), want, 1e-9), test.ShouldBeTrue)
		got, err := model.Transform(q)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqualEps(got, want, 1e-9), test.ShouldBeTrue)
	}
}

// The precomputed walk Transform used before read no offsets for a part's flattened frames.
func TestTransformFollowsTheChainIntoAPart(t *testing.T) {
	arm, err := ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	gripper, err := ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/test_mimic_gripper.json"), "")
	test.That(t, err, test.ShouldBeNil)
	parts := []*FrameSystemPart{
		{FrameConfig: NewLinkInFrame(World, spatial.NewPoseFromPoint(r3.Vector{X: 100}), "arm", nil), ModelFrame: arm},
		{FrameConfig: NewLinkInFrame("arm", spatial.NewPoseFromPoint(r3.Vector{Z: 20}), "hand", nil), ModelFrame: gripper},
		{FrameConfig: NewLinkInFrame("hand:right_finger", spatial.NewPoseFromPoint(r3.Vector{Z: 30}), "probe", nil)},
	}
	fs, err := NewFrameSystem("machine", parts, nil)
	test.That(t, err, test.ShouldBeNil)
	composed, err := NewModel("composed", fs, "probe")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(composed.DoF()), test.ShouldEqual, 7)

	rng := rand.New(rand.NewSource(17))
	for range 10 {
		inputs := RandomFrameInputs(composed, rng)
		li := NewLinearInputs()
		li.Put("arm", inputs[:6])
		li.Put("hand", inputs[6:])
		want, err := fs.Transform(li, NewPoseInFrame("probe", spatial.NewZeroPose()), World)
		test.That(t, err, test.ShouldBeNil)
		got, err := composed.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqualEps(got, want.(*PoseInFrame).Pose(), 1e-6), test.ShouldBeTrue)
	}
}

func TestChainCarriesAFrameWithSeveralInputs(t *testing.T) {
	free, err := NewPoseFrame("free", nil)
	test.That(t, err, test.ShouldBeNil)
	tip, err := NewStaticFrame("tip", spatial.NewPoseFromPoint(r3.Vector{Z: 10}))
	test.That(t, err, test.ShouldBeNil)
	model, err := NewSerialModel("free", []Frame{free, tip})
	test.That(t, err, test.ShouldBeNil)
	chain, err := model.Chain("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, chainNames(chain), test.ShouldResemble, []string{"free", "tip"})
	test.That(t, len(chain[0].Frame.DoF()), test.ShouldEqual, 7)
	test.That(t, chain[0].Offset, test.ShouldEqual, 0)
	_, revolute := chain[0].Frame.(RevoluteFrame)
	_, prismatic := chain[0].Frame.(PrismaticFrame)
	test.That(t, revolute || prismatic, test.ShouldBeFalse)
	rng := rand.New(rand.NewSource(19))
	for range 10 {
		inputs := RandomFrameInputs(model, rng)
		want, err := model.Transform(inputs)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqualEps(composeChain(t, chain, inputs), want, 1e-6), test.ShouldBeTrue)
	}
}
