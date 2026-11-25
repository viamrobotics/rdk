package motionplan

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestGetFramePosesFromTrajectory(t *testing.T) {
	arm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	toolOffset := spatialmath.NewPoseFromPoint(r3.Vector{Z: 100})
	tool, err := referenceframe.NewStaticFrame("tool", toolOffset)
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(arm, fs.World()), test.ShouldBeNil)
	test.That(t, fs.AddFrame(tool, arm), test.ShouldBeNil)

	traj := Trajectory{
		{"arm": []float64{0, 0, 0, 0, 0, 0}},
		{"arm": []float64{0.1, -0.2, 0.3, 0.1, 0.2, -0.1}},
		{"arm": []float64{0.4, 0.1, -0.2, 0.3, -0.1, 0.5}},
	}

	t.Run("matches SimplePlan path for an input-bearing frame", func(t *testing.T) {
		got, err := GetFramePosesFromTrajectory(traj, fs, "arm")
		test.That(t, err, test.ShouldBeNil)

		linear := make([]*referenceframe.LinearInputs, 0, len(traj))
		for _, step := range traj {
			linear = append(linear, step.ToLinearInputs())
		}
		plan, err := NewSimplePlanFromTrajectory(linear, fs)
		test.That(t, err, test.ShouldBeNil)
		want, err := plan.Path().GetFramePoses("arm")
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(got), test.ShouldEqual, len(want))
		for i := range want {
			test.That(t, spatialmath.PoseAlmostEqual(got[i], want[i]), test.ShouldBeTrue)
		}
	})

	t.Run("resolves a static frame with no trajectory inputs", func(t *testing.T) {
		armPoses, err := GetFramePosesFromTrajectory(traj, fs, "arm")
		test.That(t, err, test.ShouldBeNil)
		toolPoses, err := GetFramePosesFromTrajectory(traj, fs, "tool")
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(toolPoses), test.ShouldEqual, len(traj))
		for i := range toolPoses {
			want := spatialmath.Compose(armPoses[i], toolOffset)
			test.That(t, spatialmath.PoseAlmostEqual(toolPoses[i], want), test.ShouldBeTrue)
		}
	})

	t.Run("unknown frame errors", func(t *testing.T) {
		_, err := GetFramePosesFromTrajectory(traj, fs, "nope")
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("empty trajectory returns no poses", func(t *testing.T) {
		poses, err := GetFramePosesFromTrajectory(Trajectory{}, fs, "tool")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, poses, test.ShouldBeEmpty)
	})
}
