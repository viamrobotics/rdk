package armplanning

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/viam-labs/motion-tools/client/client"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestPoseCloudPlanning(t *testing.T) {
	const renderPoses = false
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs := referenceframe.NewEmptyFrameSystem("huh")
	lite6, err := referenceframe.ParseModelJSONFile(
		utils.ResolveFile("components/arm/sim/kinematics/lite6.json"), "lite6")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(lite6, fs.World())
	test.That(t, err, test.ShouldBeNil)

	gripperOffset, err := referenceframe.NewStaticFrame(
		"gripper_offset", spatialmath.NewPoseFromPoint(r3.Vector{Z: 40}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gripperOffset, lite6)
	test.That(t, err, test.ShouldBeNil)

	gripper, err := referenceframe.ParseModelJSONFile(
		utils.ResolveFile("referenceframe/testfiles/test_gripper.json"), "gripper")
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(gripper, gripperOffset)
	test.That(t, err, test.ShouldBeNil)

	glassGeometry, err := spatialmath.NewBox(
		spatialmath.NewPose(
			r3.Vector{X: 400, Y: -150, Z: 60},
			&spatialmath.OrientationVectorDegrees{OX: -.2, OY: 0.3, OZ: 0.6},
		), r3.Vector{X: 20, Y: 60, Z: 100}, "glass")
	test.That(t, err, test.ShouldBeNil)
	glassFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"glass", spatialmath.NewPose(
			r3.Vector{X: 400, Y: -150, Z: 60},
			&spatialmath.OrientationVectorDegrees{OX: -.2, OY: 0.3, OZ: 0.6},
		),
		glassGeometry)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(glassFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)

	if renderPoses {
		err = client.RemoveAllSpatialObjects()
		test.That(t, err, test.ShouldBeNil)
		err = client.DrawFrameSystem(fs, referenceframe.FrameSystemInputs{
			"lite6":   []referenceframe.Input{0, 0.5, 1, 0, 0, 0},
			"gripper": []referenceframe.Input{25, 25},
		})
		test.That(t, err, test.ShouldBeNil)
	}

	// Plan for a "bad" goal with no leeway. Because the goal is right on the glass, the arm and
	// glass would be in collision. Ideally the Z would be backed out by ~30. We should get a "no IK
	// solutions" error.
	_, _, err = PlanMotion(ctx, logger.Sublogger("cloud-planning-fails"), &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			NewPlanState(referenceframe.FrameSystemPoses{
				"gripper": referenceframe.NewPoseInFrame(
					"glass", spatialmath.NewPoseFromOrientation(
						&spatialmath.EulerAngles{Roll: math.Pi, Yaw: math.Pi / 2})),
			}, nil),
		},
		StartState: NewPlanState(nil, referenceframe.FrameSystemInputs{
			"lite6":   []referenceframe.Input{0, 0, 0, 0, 0, 0},
			"gripper": []referenceframe.Input{25, 25},
		}),
	})
	test.That(t, err, test.ShouldNotBeNil)
	var ikErr *IkConstraintError
	// E.g: all IK solutions failed constraints. Failures: { robot constraint: violation between
	// glass and lite6:wrist_link geometries: 90.09% }
	test.That(t, errors.As(err, &ikErr), test.ShouldBeTrue)

	// Plan for the same goal with a big leeway. IK finds a solution here due to the relaxed goal.
	plan, _, err := PlanMotion(ctx, logger.Sublogger("cloud-planning-works"), &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			NewPlanState(referenceframe.FrameSystemPoses{
				"gripper": referenceframe.NewPoseInFrameWithGoalCloud(
					"glass", spatialmath.NewPoseFromOrientation(
						&spatialmath.EulerAngles{Roll: math.Pi, Yaw: math.Pi / 2}),
					&referenceframe.PoseCloud{
						X: 10, Y: 10, Z: 40, OX: 1.0, OY: 1.0, Theta: 45,
					},
				),
			}, nil),
		},
		StartState: NewPlanState(nil, referenceframe.FrameSystemInputs{
			"lite6":   []referenceframe.Input{0, 0, 0, 0, 0, 0},
			"gripper": []referenceframe.Input{25, 25},
		}),
		PlannerOptions: &PlannerOptions{
			// By using a larger defaultTimeout, IK will get more time than the typical one second.
			Timeout: defaultTimeout + 1,
		},
	})
	test.That(t, err, test.ShouldBeNil)

	if renderPoses {
		client.DrawFrameSystem(fs, plan.Trajectory()[len(plan.Trajectory())-1])
	}
}
