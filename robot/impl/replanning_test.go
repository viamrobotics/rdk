package robotimpl

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang/geo/r3"
	viz "github.com/viam-labs/motion-tools/client/client"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/sim"
	"go.viam.com/rdk/components/camera"
	fakecamera "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	motionservice "go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	visionservice "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/vision"
)

type customSegmenter struct {
	mu      sync.Mutex
	objects []*vision.Object
}

func (cs *customSegmenter) GetObjects(ctx context.Context, src camera.Camera) ([]*vision.Object, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.objects, nil
}

type replanningTestObstacleServiceConfig struct{}

func (rsConfig *replanningTestObstacleServiceConfig) Validate(path string) ([]string, []string, error) {
	return nil, nil, nil
}

func newObstacleVisionService(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
	segmenter *customSegmenter,
	defaultCameraName string,
) (visionservice.Service, error) {
	return visionservice.NewService(conf.ResourceName(), deps, logger, nil, nil, nil,
		segmenter.GetObjects,
		defaultCameraName)
}

func TestMotionServiceReplanningOnObstacle(t *testing.T) {
	// TODO: Test currently requires the motion-tools visualization webserver to be running with a
	// connected client.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// Create a "mocked" segmenter. This will back the `GetObjectPointClouds` method of the vision
	// service that a motion service request can use. The motion service will poll
	// `GetObjectPointClouds` and potentially halt a plan that is currently executing because a new
	// obstacle appeared, invalidating assumptions the plan had made.
	segmenter := &customSegmenter{}
	replanningTestObstacleServiceModel := resource.ModelNamespaceRDK.WithFamily("testing").WithModel("replanning")
	resource.RegisterService(
		visionservice.API,
		replanningTestObstacleServiceModel,
		resource.Registration[visionservice.Service, *replanningTestObstacleServiceConfig]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
			) (visionservice.Service, error) {
				return newObstacleVisionService(ctx, deps, conf, logger, segmenter, "camera")
			},
		})

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
			{
				Name:  "camera",
				API:   camera.API,
				Model: fakecamera.Model,
				Frame: &referenceframe.LinkConfig{
					Geometry: &spatialmath.GeometryConfig{
						Type: spatialmath.BoxType,
						X:    10, Y: 20, Z: 10,
						Label: "cameraBox",
					},
					// The arm's `wrist_link` has an X length of 75 and the camera's center is 5
					// away from the X border.
					Translation: r3.Vector{X: (-75 / 2) - 5, Y: 0, Z: 0},
					Parent:      "arm",
				},
				ConvertedAttributes: &fakecamera.Config{
					Width:  100,
					Height: 100,
				},
			},
		},
		Services: []resource.Config{
			{
				Name:  "motionService",
				API:   motionservice.API,
				Model: resource.DefaultServiceModel,
			},
			{
				Name:      "obstacleService",
				API:       visionservice.API,
				Model:     replanningTestObstacleServiceModel,
				DependsOn: []string{"camera"},
			},
		},
	}

	robot := setupLocalRobot(t, ctx, cfg, logger)

	// Assert all of the components/services are properly instantiated.
	armI, err := arm.FromProvider(robot, "arm")
	test.That(t, err, test.ShouldBeNil)
	simArm := armI.(*sim.SimulatedArm)

	camera, err := camera.FromProvider(robot, "camera")
	test.That(t, err, test.ShouldBeNil)
	_ = camera

	motion, err := motionservice.FromProvider(robot, "motionService")
	test.That(t, err, test.ShouldBeNil)
	_ = motion

	obstacleService, err := visionservice.FromProvider(robot, "obstacleService")
	test.That(t, err, test.ShouldBeNil)
	_ = obstacleService

	robotFsI, err := robot.GetResource(framesystem.InternalServiceName)
	test.That(t, err, test.ShouldBeNil)
	robotFs := robotFsI.(framesystem.Service)

	// Assert the vision service `GetObjectPointClouds` does not return any obstacles.
	objects, err := obstacleService.GetObjectPointClouds(ctx, "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objects, test.ShouldHaveLength, 0)

	startArmPose, err := armI.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// Create a goal 200 points away on the Z-coordinate.
	armGoal := spatialmath.Compose(startArmPose, spatialmath.NewPoseFromPoint(r3.Vector{X: -300, Y: 0, Z: 0}))
	logger.Info("Start arm pose:", startArmPose, "Goal:", armGoal)

	emptyWorldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{}, nil)
	test.That(t, err, test.ShouldBeNil)

	moveRequest := &motionservice.MoveReq{
		ComponentName: "arm",
		Destination:   referenceframe.NewPoseInFrame("world", armGoal),
		WorldState:    emptyWorldState,
		Extra: map[string]any{
			"obstacleVisionService": "obstacleService",
		},
	}

	// Turn that request into the "map" that the motion service `DoCommand` accepts. Yes, it is this
	// many steps.
	moveRequestProto, err := moveRequest.ToProto("motionService")
	test.That(t, err, test.ShouldBeNil)

	moveRequestProtoBytes, err := protojson.Marshal(moveRequestProto)
	test.That(t, err, test.ShouldBeNil)

	testClock, ctx := errgroup.WithContext(ctx)
	defer testClock.Wait()

	doneMovingCtx, doneMoving := context.WithCancel(ctx)
	start := time.Now()
	testClock.Go(func() error {
		for doneMovingCtx.Err() == nil {
			now := time.Now()
			simArm.UpdateForTime(now)

			// Introduce an obstacle just after the beginning of the simulation. The obstacle is
			// halfway between the start and endpoint.
			if time.Since(start) > 1000*time.Millisecond {
				segmenter.mu.Lock()
				if len(segmenter.objects) == 0 {
					// Add a 10x10x10 cube obstacle centered between the arm's start position and the goal. Assert
					// the vision service `GetObjectPointClouds` returns this obstacle.
					logger.Info("Adding object")
					segmenter.objects = []*vision.Object{
						&vision.Object{
							PointCloud: pointcloud.NewBasicPointCloud(10),
							Geometry: spatialmath.NewBoxGoodInput(
								spatialmath.Compose(startArmPose, spatialmath.NewPoseFromPoint(r3.Vector{X: -150, Y: 0, Z: 0})),
								r3.Vector{X: 50, Y: 50, Z: 50},
								"box1"),
						},
					}

				}
				segmenter.mu.Unlock()

				// `GetObjectPointClouds` acquires the `segmenter.mu`. Assert after releasing lock
				// held for modification.
				objects, err = obstacleService.GetObjectPointClouds(ctx, "", nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, objects, test.ShouldHaveLength, 1)
			}

			time.Sleep(time.Millisecond)
		}

		logger.Info("Exiting at:", time.Since(start))
		return nil
	})

	testClock.Go(func() error {
		fs, err := framesystem.NewFromService(doneMovingCtx, robotFs, nil)
		if err != nil {
			fmt.Println("Err:", err)
			return err
		}

		return visualize(doneMovingCtx, fs, moveRequest, obstacleService, armI, armGoal, logger)
	})

	moveCmdProto, err := protoutils.StructToStructPb(map[string]any{
		"replannable": string(moveRequestProtoBytes),
		//"plan": string(moveRequestProtoBytes),
	})
	test.That(t, err, test.ShouldBeNil)

	resMap, err := motion.DoCommand(ctx, moveCmdProto.AsMap())
	test.That(t, err, test.ShouldBeNil)

	var trajectory motionplan.Trajectory
	err = mapstructure.Decode(resMap["plan"], &trajectory)
	test.That(t, err, test.ShouldBeNil)
	logger.Infof("Num waypoints: %v Traj: %v", len(trajectory), trajectory)

	doneMoving()
	test.That(t, testClock.Wait(), test.ShouldBeNil)
}

func visualize(
	ctx context.Context,
	fs *referenceframe.FrameSystem,
	req *motion.MoveReq,
	obstacleVisionService visionservice.Service,
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

	var logOnce sync.Once
	for ctx.Err() == nil {
		armInputs, err := arm.CurrentInputs(ctx)
		if err != nil {
			panic(err)
		}

		fsi := make(referenceframe.FrameSystemInputs)
		fsi["arm"] = armInputs

		currObjects, err := obstacleVisionService.GetObjectPointClouds(ctx, "", nil)
		if len(currObjects) > 0 {
			logOnce.Do(func() {
				logger.Info("Obstacle appeared")
			})
		}

		geoms := make([]spatialmath.Geometry, len(currObjects))
		for idx, newObstacle := range currObjects {
			geoms[idx] = newObstacle.Geometry
		}
		geomsInFrame := referenceframe.NewGeometriesInFrame("world", geoms)
		worldState := req.WorldState.Merge(geomsInFrame)

		if geoms, err := worldState.ObstaclesInWorldFrame(fs, fsi); err != nil {
			panic(err)
		} else if len(geoms.Geometries()) > 0 {
			// `DrawWorldState` just draws the obstacles. I think the FrameSystem/Path are necessary
			// because obstacles can be in terms of reference frames contained within the frame
			// system. Such as a camera attached to an arm.
			if err := viz.DrawWorldState(worldState, fs, fsi); err != nil {
				panic(err)
			}
		}

		if err := viz.DrawFrameSystem(fs, fsi); err != nil {
			panic(err)
		}

		time.Sleep(10 * time.Millisecond)
	}

	return nil
}
