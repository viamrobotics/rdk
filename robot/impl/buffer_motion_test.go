package robotimpl

import (
	"context"
	"sync/atomic"
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
	msBuiltin "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
	"golang.org/x/sync/errgroup"
)

func TestBufferMoveRequests(t *testing.T) {
	// TODO: Test currently requires the motion-tools visualization webserver to be running with a
	// connected client.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	_ = msBuiltin.Config{}

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
					Speed: 0.73, // radians per second
				},
			},
		},
		Services: []resource.Config{
			{
				Name:                "motionService",
				API:                 motionservice.API,
				Model:               resource.DefaultServiceModel,
				ConvertedAttributes: nil,
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

	var armGoal atomic.Pointer[spatialmath.Pose]
	testClock.Go(func() error {
		fs, err := framesystem.NewFromService(ctx, robotFs, nil)
		if err != nil {
			return err
		}

		return visualizeLess(ctx, fs, armI, &armGoal, logger)
	})

	// Init position. Basically the same as all 0s.
	err = simArm.MoveToJointPositions(ctx, []referenceframe.Input{
		-2.6070722014992498e-05,
		0.0003121367481071502,
		0.0004811931576114148,
		-1.65975907293614e-05,
		0.00022126760450191796,
		0.0006661340012215079,
	}, nil)
	test.That(t, err, test.ShouldBeNil)

	logger.Info("Start arm pose:", startArmPose)
	type scenario struct {
		pose spatialmath.Pose

		sleepWaitingReceived time.Duration
		preMPWait            time.Duration
		mpTime               time.Duration
	}
	scenes := []scenario{
		{
			pose: spatialmath.NewPose(
				r3.Vector{X: 87.2858, Y: -0.0854, Z: 154.3494},
				&spatialmath.OrientationVector{
					OX:    5.221119372527562e-05,
					OY:    1.0866956811172634e-25,
					OZ:    -0.9999999986369956,
					Theta: 0.038709437428360555,
				}),
			sleepWaitingReceived: 0,
			preMPWait:            26 * time.Millisecond,
			mpTime:               37 * time.Millisecond,
		},
		{
			pose: spatialmath.NewPose(
				r3.Vector{X: 88.5368, Y: -0.1742, Z: 155.2143},
				&spatialmath.OrientationVector{
					OX:    5.221119372527562e-05,
					OY:    1.0866956811172634e-25,
					OZ:    -0.9999999986369956,
					Theta: 0.038709437428360555,
				}),
			sleepWaitingReceived: 19 * time.Millisecond,
			preMPWait:            28 * time.Millisecond,
			mpTime:               33 * time.Millisecond,
		},
		{
			pose: spatialmath.NewPose(
				r3.Vector{X: 91.4828, Y: -0.8744, Z: 156.0627},
				&spatialmath.OrientationVector{
					OX:    5.221119372527562e-05,
					OY:    1.0866956811172634e-25,
					OZ:    -0.9999999986369956,
					Theta: 0.038709437428360555,
				}),
			sleepWaitingReceived: 28 * time.Millisecond,
			preMPWait:            19 * time.Millisecond,
			mpTime:               17 * time.Millisecond,
		},
		{
			pose: spatialmath.NewPose(
				r3.Vector{X: 125.8221, Y: -3.9480, Z: 161.79193},
				&spatialmath.OrientationVector{
					OX:    5.221119372527562e-05,
					OY:    1.0866956811172634e-25,
					OZ:    -0.9999999986369956,
					Theta: 0.038709437428360555,
				}),
			sleepWaitingReceived: 42 * time.Millisecond,
			preMPWait:            6 * time.Millisecond,
			mpTime:               99 * time.Millisecond,
		},
		{
			pose: spatialmath.NewPose(
				r3.Vector{X: 348.1348, Y: 2.4793, Z: 155.1889},
				&spatialmath.OrientationVector{
					OX:    5.221119372527562e-05,
					OY:    1.0866956811172634e-25,
					OZ:    -0.9999999986369956,
					Theta: 0.038709437428360555,
				}),
			sleepWaitingReceived: 52 * time.Millisecond,
			preMPWait:            26 * time.Millisecond,
			mpTime:               149 * time.Millisecond,
		},
	}

	for _, scene := range scenes {
		goal := scene.pose
		time.Sleep(scene.sleepWaitingReceived)
		time.Sleep(scene.preMPWait)
		armGoal.Store(&goal)
		moveRequest := motionservice.MoveReq{
			ComponentName: "arm",
			Destination:   referenceframe.NewPoseInFrame("world", goal),
			Extra: map[string]any{
				"ensureMPTimeSpentMS": scene.mpTime.Milliseconds(),
			},
		}

		_, err = motion.Move(ctx, moveRequest)
		test.That(t, err, test.ShouldBeNil)
	}
}

func visualizeLess(
	ctx context.Context,
	fs *referenceframe.FrameSystem,
	arm arm.Arm,
	goalPosePtr *atomic.Pointer[spatialmath.Pose],
	logger logging.Logger,
) error {
	if err := viz.RemoveAllSpatialObjects(); err != nil {
		panic(err)
	}

	for ctx.Err() == nil {
		armInputs, err := arm.CurrentInputs(ctx)
		if err != nil {
			panic(err)
		}

		const arrowHeadAtPose = true
		if goalPose := goalPosePtr.Load(); goalPose != nil {
			if err := viz.DrawPoses([]spatialmath.Pose{*goalPose}, []string{"blue"}, arrowHeadAtPose); err != nil {
				panic(err)
			}
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
