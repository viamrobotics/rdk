package armplanning

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// TestOrientationCloudPlanning carries an upright "cup" between two poses whose orientations
// differ by a large spin about the cup's own axis. An arc-based orientation constraint would
// force the path to track that spin; the cloud instead lets the spin float freely while pinning
// the cup's axis to within ~10 degrees of upright at every waypoint.
func TestOrientationCloudPlanning(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs := referenceframe.NewEmptyFrameSystem("cup")
	arm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/xarm6.json"), "arm")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(arm, fs.World()), test.ShouldBeNil)

	startInputs := referenceframe.FrameSystemInputs{"arm": {0, -0.4, -0.6, 0, 1.0, 0}}
	startTF, err := fs.Transform(startInputs.ToLinearInputs(), referenceframe.NewZeroPoseInFrame("arm"), referenceframe.World)
	test.That(t, err, test.ShouldBeNil)
	startPose := startTF.(*referenceframe.PoseInFrame).Pose()

	// The goal keeps the cup's axis where it is and spins it 150 degrees about that axis while
	// moving it sideways.
	goalPose := spatialmath.Compose(
		spatialmath.NewPose(startPose.Point().Add(r3.Vector{X: -60, Y: 160, Z: 40}), startPose.Orientation()),
		spatialmath.NewPoseFromOrientation(&spatialmath.EulerAngles{Yaw: utils.DegToRad(150)}),
	)
	cup := motionplan.OrientationCloudConstraint{
		OrientationCloud: referenceframe.OrientationCloud{OX: 0.17, OY: 0.17, OZ: 0.015, Theta: 180},
	}

	constraints := motionplan.NewEmptyConstraints()
	constraints.AddOrientationCloudConstraint(cup)
	// Retreat keypoints get the cloud's inscribed angle as orientation slack.
	test.That(t, midOrientationSlackDegs(constraints), test.ShouldAlmostEqual, cup.InscribedAngleDegs(), 1e-9)

	plan, _, err := PlanMotion(ctx, logger, &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{NewPlanState(referenceframe.FrameSystemPoses{
			"arm": referenceframe.NewPoseInFrame(referenceframe.World, goalPose),
		}, nil)},
		StartState:     NewPlanState(nil, startInputs),
		Constraints:    constraints,
		PlannerOptions: NewBasicPlannerOptions(),
	})
	test.That(t, err, test.ShouldBeNil)

	traj := plan.Trajectory()
	test.That(t, len(traj), test.ShouldBeGreaterThan, 1)
	for i, step := range traj {
		tf, err := fs.Transform(step.ToLinearInputs(), referenceframe.NewZeroPoseInFrame("arm"), referenceframe.World)
		test.That(t, err, test.ShouldBeNil)
		o := tf.(*referenceframe.PoseInFrame).Pose().Orientation()
		test.That(t, cup.OrientationInCloud(goalPose.Orientation(), o), test.ShouldBeTrue)
		logger.Debugf("step %d excess %0.3f deg", i, cup.Excess(goalPose.Orientation(), o))
	}
}
