package armplanning

import (
	"maps"
	"testing"

	"go.viam.com/test"

	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func frozenTestStart() frame.FrameSystemInputs {
	return frame.FrameSystemInputs{
		"gantryX": {300},
		"gantryY": {-50},
		"xArm6":   {0.1, 0.2, -0.3, 0.4, 0.5, 0.6},
		"UR5e":    {0.3, -0.5, 0.4, 0.1, 0.2, -0.1},
	}
}

func withoutFrames(in frame.FrameSystemInputs, names ...string) frame.FrameSystemInputs {
	out := maps.Clone(in)
	for _, n := range names {
		delete(out, n)
	}
	return out
}

func geometriesByLabel(t *testing.T, fs *frame.FrameSystem, inputs frame.FrameSystemInputs) map[string]spatialmath.Geometry {
	t.Helper()
	gifs, err := frame.FrameSystemGeometries(fs, inputs)
	test.That(t, err, test.ShouldBeNil)
	out := map[string]spatialmath.Geometry{}
	for _, gif := range gifs {
		for _, g := range gif.Geometries() {
			out[g.Label()] = g
		}
	}
	return out
}

func TestFreezeFramesPreservesGeometry(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenTestStart()
	frozenFS, _, err := freezeFrames(fs, []string{"gantryX", "xArm6"}, start)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(frozenFS.Frame("gantryX").DoF()), test.ShouldEqual, 0)
	test.That(t, len(frozenFS.Frame("xArm6").DoF()), test.ShouldEqual, 0)

	before := geometriesByLabel(t, fs, start)
	after := geometriesByLabel(t, frozenFS, withoutFrames(start, "gantryX", "xArm6"))
	test.That(t, len(before), test.ShouldBeGreaterThan, 0)
	for label, g := range before {
		frozen, ok := after[label]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, spatialmath.PoseAlmostEqual(g.Pose(), frozen.Pose()), test.ShouldBeTrue)
	}
}

func TestFreezeFramesKeepsChildPoses(t *testing.T) {
	fs := makeTestFS(t)
	start := frozenTestStart()
	frozenFS, _, err := freezeFrames(fs, []string{"UR5e"}, start)
	test.That(t, err, test.ShouldBeNil)

	cameraPose := func(fs *frame.FrameSystem, in frame.FrameSystemInputs) spatialmath.Pose {
		tf, err := fs.Transform(in.ToLinearInputs(), frame.NewZeroPoseInFrame("urCamera"), frame.World)
		test.That(t, err, test.ShouldBeNil)
		return tf.(*frame.PoseInFrame).Pose()
	}
	test.That(t, spatialmath.PoseAlmostEqual(
		cameraPose(fs, start),
		cameraPose(frozenFS, withoutFrames(start, "UR5e")),
	), test.ShouldBeTrue)
}

func TestFreezeFramesReturnsInputs(t *testing.T) {
	start := frozenTestStart()
	_, frozen, err := freezeFrames(makeTestFS(t), []string{"gantryX", "xArm6"}, start)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frozen, test.ShouldResemble, frame.FrameSystemInputs{
		"gantryX": start["gantryX"],
		"xArm6":   start["xArm6"],
	})
}

func TestFreezeFramesErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		frames []string
		start  frame.FrameSystemInputs
		errHas string
	}{
		{"unknown frame", []string{"nope"}, frozenTestStart(), "nope"},
		{"zero DoF frame", []string{"urOffset"}, frozenTestStart(), "urOffset"},
		{"missing from start", []string{"gantryY"}, withoutFrames(frozenTestStart(), "gantryY"), "gantryY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := freezeFrames(makeTestFS(t), tc.frames, tc.start)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errHas)
		})
	}
}

func TestFreezeFramesDoesNotMutateInput(t *testing.T) {
	fs := makeTestFS(t)
	_, _, err := freezeFrames(fs, []string{"gantryX"}, frozenTestStart())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(fs.Frame("gantryX").DoF()), test.ShouldEqual, 1)
}
