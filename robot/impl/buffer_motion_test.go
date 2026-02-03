package robotimpl

import (
	"context"
	"testing"
	"time"

	viz "github.com/viam-labs/motion-tools/client/client"
	motionservice "go.viam.com/rdk/services/motion"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/sim"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
	"golang.org/x/sync/errgroup"
)

func TestBufferMoveRequests(t *testing.T) {
	// TODO: Test currently requires the motion-tools visualization webserver to be running with a
	// connected client.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// Create a robot with the minimal components for replanning. An arm, a camera, a motion service
	// and a vision detector.
	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm",
				API:   arm.API,
				Model: sim.Model,
				Frame: &referenceframe.LinkConfig{
					Translation: r3.Vector{X: 0, Y: 0, Z: 0},
					Parent:      "world",
				},
				ConvertedAttributes: &sim.Config{
					Model: "lite6",
					// At a `Speed` of 1.0, the test takes about two simulated seconds to complete.
					Speed: 0.2,
				},
			},
		},
		Services: []resource.Config{
			{
				Name:  "motionService",
				API:   motionservice.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}

	robot := setupLocalRobot(t, ctx, cfg, logger)

	// Assert all of the components/services are properly instantiated.
	armI, err := arm.FromProvider(robot, "arm")
	test.That(t, err, test.ShouldBeNil)
	simArm := armI.(*sim.SimulatedArm)

	motion, err := motionservice.FromProvider(robot, "motionService")
	test.That(t, err, test.ShouldBeNil)
	_ = motion

	startArmPose, err := armI.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// Create a goal 200 points away on the Z-coordinate.
	armGoal := spatialmath.Compose(startArmPose, spatialmath.NewPoseFromPoint(r3.Vector{X: -300, Y: 0, Z: 0}))
	logger.Info("Start arm pose:", startArmPose, "Goal:", armGoal)

	testClock := &errgroup.Group{}
	defer testClock.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	robotFsI, err := robot.GetResource(framesystem.InternalServiceName)
	test.That(t, err, test.ShouldBeNil)
	robotFs := robotFsI.(framesystem.Service)

	testClock.Go(func() error {
		for ctx.Err() == nil {
			now := time.Now()
			simArm.UpdateForTime(now)
		}

		return nil
	})

	testClock.Go(func() error {
		fs, err := framesystem.NewFromService(ctx, robotFs, nil)
		if err != nil {
			return err
		}

		return visualize(ctx, fs, armI, armGoal, logger)
	})

	moveRequest := motionservice.MoveReq{
		ComponentName: "arm",
		Destination:   referenceframe.NewPoseInFrame("world", armGoal),
	}

	_, err = motion.Move(ctx, moveRequest)
	test.That(t, err, test.ShouldBeNil)
}

func visualize(
	ctx context.Context,
	fs *referenceframe.FrameSystem,
	arm arm.Arm,
	goalPose spatialmath.Pose,
	logger logging.Logger,
) error {
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		panic(err)
	}

	const arrowHeadAtPose = true
	if err := viz.DrawPoses([]spatialmath.Pose{goalPose}, []string{"blue"}, arrowHeadAtPose); err != nil {
		panic(err)
	}

	for ctx.Err() == nil {
		armInputs, err := arm.CurrentInputs(ctx)
		if err != nil {
			panic(err)
		}

		fsi := make(referenceframe.FrameSystemInputs)
		fsi["arm"] = armInputs

		if err := viz.DrawFrameSystem(fs, fsi); err != nil {
			panic(err)
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}
