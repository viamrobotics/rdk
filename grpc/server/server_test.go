package server_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/grpc/client"
	grpcserver "go.viam.com/core/grpc/server"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	servicepkg "go.viam.com/core/services"
	"go.viam.com/core/servo"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/testutils/inject"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newServer() (pb.RobotServiceServer, *inject.Robot) {
	injectRobot := &inject.Robot{}
	return grpcserver.New(injectRobot), injectRobot
}

var emptyStatus = &pb.StatusResponse{
	Status: &pb.Status{
		Arms: map[string]*pb.ArmStatus{
			"arm1": {
				GridPosition: &pb.Pose{
					X:     0.0,
					Y:     0.0,
					Z:     0.0,
					Theta: 0.0,
					OX:    1.0,
					OY:    0.0,
					OZ:    0.0,
				},
				JointPositions: &pb.JointPositions{
					Degrees: []float64{0, 0, 0, 0, 0, 0},
				},
			},
		},
		Bases: map[string]bool{
			"base1": true,
		},
		Grippers: map[string]bool{
			"gripper1": true,
		},
		Cameras: map[string]bool{
			"camera1": true,
		},
		Lidars: map[string]bool{
			"lidar1": true,
		},
		Servos: map[string]*pb.ServoStatus{
			"servo1": {},
		},
		Motors: map[string]*pb.MotorStatus{
			"motor1": {},
		},
		InputControllers: map[string]bool{
			"inputController1": true,
		},
		Boards: map[string]*pb.BoardStatus{
			"board1": {
				Analogs: map[string]*pb.AnalogStatus{
					"analog1": {},
				},
				DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
					"encoder": {},
				},
			},
		},
	},
}

func TestServer(t *testing.T) {
	t.Run("Status", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return nil, err1
		}
		_, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldEqual, err1)

		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return emptyStatus.Status, nil
		}
		statusResp, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusResp, test.ShouldResemble, emptyStatus)
	})

	t.Run("Config", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
			return nil, err1
		}
		_, err := server.Config(context.Background(), &pb.ConfigRequest{})
		test.That(t, err, test.ShouldEqual, err1)

		cfg := config.Config{
			Components: []config.Component{
				{
					Name: "a",
					Type: config.ComponentTypeArm,
					Frame: &config.Frame{
						Parent: "b",
					},
				},
			},
		}
		injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
			return &cfg, nil
		}
		statusResp, err := server.Config(context.Background(), &pb.ConfigRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statusResp.Components), test.ShouldEqual, len(cfg.Components))
		test.That(t, statusResp.Components[0].Name, test.ShouldEqual, cfg.Components[0].Name)
		test.That(t, statusResp.Components[0].Parent, test.ShouldEqual, cfg.Components[0].Frame.Parent)
		test.That(t, statusResp.Components[0].Type, test.ShouldResemble, string(cfg.Components[0].Type))
	})

	t.Run("FrameServiceConfig", func(t *testing.T) {
		server, injectRobot := newServer()

		// create a basic frame system
		fsConfigs := []*config.FrameSystemPart{
			{
				Name: "frame1",
				FrameConfig: &config.Frame{
					Parent:      referenceframe.World,
					Translation: config.Translation{1, 2, 3},
					Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
				},
			},
			{
				Name: "frame2",
				FrameConfig: &config.Frame{
					Parent:      "frame1",
					Translation: config.Translation{1, 2, 3},
				},
			},
		}
		fss := &inject.FrameSystemService{}
		fss.FrameSystemConfigFunc = func(ctx context.Context) ([]*config.FrameSystemPart, error) {
			return fsConfigs, nil
		}
		// set up the robot without a frame system service
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			services := make(map[string]interface{})
			service, ok := services[name]
			return service, ok
		}
		_, err := server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("no service named \"frame_system\""))

		// set up the robot with something that is not a framesystem service
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			services := make(map[string]interface{})
			services[servicepkg.FrameSystemName] = nil
			service, ok := services[name]
			return service, ok
		}
		_, err = server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("service is not a framesystem.Service"))

		// set up the robot with the frame system
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			services := make(map[string]interface{})
			services[servicepkg.FrameSystemName] = fss
			service, ok := services[name]
			return service, ok
		}

		fssResp, err := server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(fssResp.FrameSystemConfigs), test.ShouldEqual, len(fsConfigs))
		test.That(t, fssResp.FrameSystemConfigs[0].Name, test.ShouldEqual, fsConfigs[0].Name)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Parent, test.ShouldEqual, fsConfigs[0].FrameConfig.Parent)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.X, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Translation.X)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Y, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Translation.Y)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Z, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Translation.Z)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OX, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OX)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OY, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OY)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OZ, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OZ)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Theta, test.ShouldAlmostEqual, fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().Theta)
		test.That(t, fssResp.FrameSystemConfigs[0].ModelJson, test.ShouldEqual, fsConfigs[0].ModelFrameConfig)
	})

	t.Run("ObjectManipulation", func(t *testing.T) {
		server, injectRobot := newServer()

		// set up the robot without an objectmanipulation service
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			services := make(map[string]interface{})
			service, ok := services[name]
			return service, ok
		}
		_, err := server.ObjectManipulationServiceDoGrab(context.Background(), &pb.ObjectManipulationServiceDoGrabRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("no objectmanipulation service"))

		// set up the robot with something that is not an objectmanipulation service
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			services := make(map[string]interface{})
			services[servicepkg.ObjectManipulationServiceName] = nil
			service, ok := services[name]
			return service, ok
		}
		_, err = server.ObjectManipulationServiceDoGrab(context.Background(), &pb.ObjectManipulationServiceDoGrabRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("service is not a objectmanipulation service"))

		// pass on dograb error
		passedErr := errors.New("fake dograb error")
		omSvc := &inject.ObjectManipulationService{}
		injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
			return omSvc, true
		}
		omSvc.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
			return false, passedErr
		}
		req := &pb.ObjectManipulationServiceDoGrabRequest{
			CameraName:  "fakeC",
			GripperName: "fakeG",
			ArmName:     "fakeA",
			CameraPoint: &pb.Vector3{X: 0, Y: 0, Z: 0},
		}
		_, err = server.ObjectManipulationServiceDoGrab(context.Background(), req)
		test.That(t, err, test.ShouldBeError, passedErr)

		// returns response
		omSvc.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
			return true, nil
		}
		resp, err := server.ObjectManipulationServiceDoGrab(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetHasGrabbed(), test.ShouldBeTrue)

	})

	t.Run("StatusStream", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return nil, err1
		}
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StatusStreamResponse)
		streamServer := &robotServiceStatusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}
		err := server.StatusStream(&pb.StatusStreamRequest{
			Every: durationpb.New(time.Second),
		}, streamServer)
		test.That(t, err, test.ShouldEqual, err1)

		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return emptyStatus.Status, nil
		}
		streamServer.fail = true
		dur := 100 * time.Millisecond
		err = server.StatusStream(&pb.StatusStreamRequest{
			Every: durationpb.New(dur),
		}, streamServer)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")

		streamServer.fail = false
		var streamErr error
		start := time.Now()
		done := make(chan struct{})
		go func() {
			streamErr = server.StatusStream(&pb.StatusStreamRequest{
				Every: durationpb.New(dur),
			}, streamServer)
			close(done)
		}()
		var messages []*pb.StatusStreamResponse
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		test.That(t, messages, test.ShouldResemble, []*pb.StatusStreamResponse{
			{Status: emptyStatus.Status},
			{Status: emptyStatus.Status},
			{Status: emptyStatus.Status},
		})
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 6*dur)
		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		streamServer.ctx = timeoutCtx
		streamServer.messageCh = nil
		streamErr = server.StatusStream(&pb.StatusStreamRequest{
			Every: durationpb.New(dur),
		}, streamServer)
		test.That(t, streamErr, test.ShouldResemble, context.DeadlineExceeded)
	})

	t.Run("DoAction", func(t *testing.T) {
		server, injectRobot := newServer()
		_, err := server.DoAction(context.Background(), &pb.DoActionRequest{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown action")

		actionName := utils.RandomAlphaString(5)
		called := make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)

		actionName = utils.RandomAlphaString(5)
		called = make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			go utils.TryClose(server)
			<-ctx.Done()
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)
	})

	t.Run("Base", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BaseMoveStraight(context.Background(), &pb.BaseMoveStraightRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		_, err = server.BaseSpin(context.Background(), &pb.BaseSpinRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		_, err = server.BaseStop(context.Background(), &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		injectBase := &inject.Base{}
		injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
			return injectBase, true
		}
		var capCtx context.Context
		err1 := errors.New("whoops")
		injectBase.StopFunc = func(ctx context.Context) error {
			capCtx = ctx
			return err1
		}

		ctx := context.Background()
		_, err = server.BaseStop(ctx, &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectBase.StopFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.BaseStop(ctx, &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldBeNil)

		var capArgs []interface{}
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return 2, err1
		}
		resp, err := server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 500.0, false})
		test.That(t, resp.Success, test.ShouldBeFalse)
		test.That(t, resp.Error, test.ShouldEqual, err1.Error())
		test.That(t, resp.DistanceMillis, test.ShouldEqual, 2)

		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return distanceMillis, nil
		}
		resp, err = server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			MillisPerSec:   2.3,
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 2.3, false})
		test.That(t, resp.Success, test.ShouldBeTrue)
		test.That(t, resp.DistanceMillis, test.ShouldEqual, 1)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return 2.2, err1
		}
		spinResp, err := server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:     "base1",
			AngleDeg: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 64.0, false})
		test.That(t, spinResp.Success, test.ShouldBeFalse)
		test.That(t, spinResp.Error, test.ShouldEqual, err1.Error())
		test.That(t, spinResp.AngleDeg, test.ShouldEqual, 2.2)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return angleDeg, nil
		}
		spinResp, err = server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:       "base1",
			DegsPerSec: 20.3,
			AngleDeg:   4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 20.3, false})
		test.That(t, spinResp.Success, test.ShouldBeTrue)
		test.That(t, spinResp.AngleDeg, test.ShouldEqual, 4.5)

		injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
			capArgs = []interface{}{ctx}
			return 0, err1
		}
		_, err = server.BaseWidthMillis(ctx, &pb.BaseWidthMillisRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
			capArgs = []interface{}{ctx}
			return 2, nil
		}
		widthResp, err := server.BaseWidthMillis(ctx, &pb.BaseWidthMillisRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})
		test.That(t, widthResp.WidthMillis, test.ShouldEqual, 2)
	})

	t.Run("ArmCurrentPosition", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			return injectArm, true
		}

		err1 := errors.New("whoops")
		pos := &pb.Pose{X: 1, Y: 2, Z: 3, OX: 4, OY: 5, OZ: 6}
		injectArm.CurrentPositionFunc = func(ctx context.Context) (*pb.Pose, error) {
			return nil, err1
		}

		_, err = server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectArm.CurrentPositionFunc = func(ctx context.Context) (*pb.Pose, error) {
			return pos, nil
		}
		resp, err := server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position, test.ShouldResemble, pos)
	})

	t.Run("ArmCurrentJointPositions", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			return injectArm, true
		}

		err1 := errors.New("whoops")
		pos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
		injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
			return nil, err1
		}

		_, err = server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
			return pos, nil
		}
		resp, err := server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions, test.ShouldResemble, pos)
	})

	t.Run("ArmMoveToPosition", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			return injectArm, true
		}

		err1 := errors.New("whoops")
		var capAP *pb.Pose
		injectArm.MoveToPositionFunc = func(ctx context.Context, ap *pb.Pose) error {
			capAP = ap
			return err1
		}

		pos := &pb.Pose{X: 1, Y: 2, Z: 3, OX: 4, OY: 5, OZ: 6}
		_, err = server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAP, test.ShouldEqual, pos)

		injectArm.MoveToPositionFunc = func(ctx context.Context, ap *pb.Pose) error {
			return nil
		}
		_, err = server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ArmMoveToJointPositions", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			return injectArm, true
		}

		err1 := errors.New("whoops")
		var capJP *pb.JointPositions
		injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions) error {
			capJP = jp
			return err1
		}

		pos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
		_, err = server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capJP, test.ShouldEqual, pos)

		injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions) error {
			return nil
		}
		_, err = server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ArmJointMoveDelta", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ArmJointMoveDelta(context.Background(), &pb.ArmJointMoveDeltaRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
			return injectArm, true
		}

		var capArgs []interface{}

		err1 := errors.New("whoops")
		injectArm.JointMoveDeltaFunc = func(ctx context.Context, joint int, amount float64) error {
			capArgs = []interface{}{ctx, joint, amount}
			return err1
		}

		ctx := context.Background()
		_, err = server.ArmJointMoveDelta(ctx, &pb.ArmJointMoveDeltaRequest{
			Name:       "arm1",
			Joint:      1,
			AmountDegs: 1.23,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 1.23})

		injectArm.JointMoveDeltaFunc = func(ctx context.Context, joint int, amount float64) error {
			capArgs = []interface{}{ctx, joint, amount}
			return nil
		}
		_, err = server.ArmJointMoveDelta(ctx, &pb.ArmJointMoveDeltaRequest{
			Name:       "arm1",
			Joint:      1,
			AmountDegs: 1.23,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 1.23})
	})

	t.Run("Gripper", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
			capName = name
			return nil, false
		}

		_, err := server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
		test.That(t, capName, test.ShouldEqual, "gripper1")

		_, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
		test.That(t, capName, test.ShouldEqual, "gripper1")

		injectGripper := &inject.Gripper{}
		injectRobot.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
			return injectGripper, true
		}

		err1 := errors.New("whoops")
		injectGripper.OpenFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.OpenFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)

		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return false, err1
		}
		_, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}

		resp, err := server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeFalse)

		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		resp, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeTrue)
	})

	t.Run("BoardStatus", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}

		err1 := errors.New("whoops")
		status := &pb.BoardStatus{
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		}
		injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
			return nil, err1
		}
		_, err = server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
			return status, nil
		}
		resp, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Status, test.ShouldResemble, status)
	})

	t.Run("BoardGPIOSet", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardGPIOSet(context.Background(), &pb.BoardGPIOSetRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}

		var capArgs []interface{}
		ctx := context.Background()

		err1 := errors.New("whoops")
		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			capArgs = []interface{}{ctx, pin, high}
			return err1
		}
		_, err = server.BoardGPIOSet(ctx, &pb.BoardGPIOSetRequest{
			Name: "board1",
			Pin:  "one",
			High: true,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", true})

		injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
			capArgs = []interface{}{ctx, pin, high}
			return nil
		}
		_, err = server.BoardGPIOSet(ctx, &pb.BoardGPIOSetRequest{
			Name: "board1",
			Pin:  "one",
			High: true,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", true})
	})

	t.Run("BoardGPIOGet", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardGPIOGet(context.Background(), &pb.BoardGPIOGetRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}

		var capArgs []interface{}
		ctx := context.Background()

		err1 := errors.New("whoops")
		injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
			capArgs = []interface{}{ctx, pin}
			return false, err1
		}
		_, err = server.BoardGPIOGet(ctx, &pb.BoardGPIOGetRequest{
			Name: "board1",
			Pin:  "one",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})

		injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
			capArgs = []interface{}{ctx, pin}
			return true, nil
		}
		getResp, err := server.BoardGPIOGet(ctx, &pb.BoardGPIOGetRequest{
			Name: "board1",
			Pin:  "one",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one"})
		test.That(t, getResp.High, test.ShouldBeTrue)
	})

	t.Run("BoardPWMSet", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardPWMSet(context.Background(), &pb.BoardPWMSetRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}

		var capArgs []interface{}
		ctx := context.Background()

		err1 := errors.New("whoops")
		injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
			capArgs = []interface{}{ctx, pin, dutyCycle}
			return err1
		}
		_, err = server.BoardPWMSet(ctx, &pb.BoardPWMSetRequest{
			Name:      "board1",
			Pin:       "one",
			DutyCycle: 7,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", byte(7)})

		injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
			capArgs = []interface{}{ctx, pin, dutyCycle}
			return nil
		}
		_, err = server.BoardPWMSet(ctx, &pb.BoardPWMSetRequest{
			Name:      "board1",
			Pin:       "one",
			DutyCycle: 7,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", byte(7)})
	})

	t.Run("BoardPWMSetFrequency", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardPWMSetFrequency(context.Background(), &pb.BoardPWMSetFrequencyRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}

		var capArgs []interface{}
		ctx := context.Background()

		err1 := errors.New("whoops")
		injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
			capArgs = []interface{}{ctx, pin, freq}
			return err1
		}
		_, err = server.BoardPWMSetFrequency(ctx, &pb.BoardPWMSetFrequencyRequest{
			Name:      "board1",
			Pin:       "one",
			Frequency: 123123,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", uint(123123)})

		injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
			capArgs = []interface{}{ctx, pin, freq}
			return nil
		}
		_, err = server.BoardPWMSetFrequency(ctx, &pb.BoardPWMSetFrequencyRequest{
			Name:      "board1",
			Pin:       "one",
			Frequency: 123123,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, "one", uint(123123)})
	})

	t.Run("BoardAnalogReaderRead", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			capName = name
			return nil, false
		}

		_, err = server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
			BoardName:        "board1",
			AnalogReaderName: "analog1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown analog reader")
		test.That(t, capName, test.ShouldEqual, "analog1")

		injectAnalogReader := &inject.AnalogReader{}
		injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return injectAnalogReader, true
		}

		var capCtx context.Context
		err1 := errors.New("whoops")
		injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
			capCtx = ctx
			return 0, err1
		}
		ctx := context.Background()
		_, err = server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
			BoardName:        "board1",
			AnalogReaderName: "analog1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
			capCtx = ctx
			return 8, nil
		}
		readResp, err := server.BoardAnalogReaderRead(context.Background(), &pb.BoardAnalogReaderReadRequest{
			BoardName:        "board1",
			AnalogReaderName: "analog1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capCtx, test.ShouldEqual, ctx)
		test.That(t, readResp.Value, test.ShouldEqual, 8)
	})

	t.Run("BoardDigitalInterruptConfig", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			capName = name
			return nil, false
		}

		_, err = server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
		test.That(t, capName, test.ShouldEqual, "digital1")

		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			return injectDigitalInterrupt, true
		}

		var capCtx context.Context
		err1 := errors.New("whoops")
		injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
			capCtx = ctx
			return board.DigitalInterruptConfig{}, err1
		}
		ctx := context.Background()
		_, err = server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		theConfig := board.DigitalInterruptConfig{
			Name:    "foo",
			Pin:     "bar",
			Type:    "baz",
			Formula: "baf",
		}
		injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
			capCtx = ctx
			return theConfig, nil
		}
		configResp, err := server.BoardDigitalInterruptConfig(context.Background(), &pb.BoardDigitalInterruptConfigRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capCtx, test.ShouldEqual, ctx)
		test.That(t, client.DigitalInterruptConfigFromProto(configResp.Config), test.ShouldResemble, theConfig)
	})

	t.Run("BoardDigitalInterruptValue", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			capName = name
			return nil, false
		}

		_, err = server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
		test.That(t, capName, test.ShouldEqual, "digital1")

		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			return injectDigitalInterrupt, true
		}

		var capCtx context.Context
		err1 := errors.New("whoops")
		injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
			capCtx = ctx
			return 0, err1
		}
		ctx := context.Background()
		_, err = server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
			capCtx = ctx
			return 42, nil
		}
		valueResp, err := server.BoardDigitalInterruptValue(context.Background(), &pb.BoardDigitalInterruptValueRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capCtx, test.ShouldEqual, ctx)
		test.That(t, valueResp.Value, test.ShouldEqual, 42)
	})

	t.Run("BoardDigitalInterruptTick", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		_, err := server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return injectBoard, true
		}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			capName = name
			return nil, false
		}

		_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown digital interrupt")
		test.That(t, capName, test.ShouldEqual, "digital1")

		injectDigitalInterrupt := &inject.DigitalInterrupt{}
		injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
			return injectDigitalInterrupt, true
		}

		var capArgs []interface{}
		err1 := errors.New("whoops")
		injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
			capArgs = []interface{}{ctx, high, nanos}
			return err1
		}
		ctx := context.Background()
		_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
			High:                 true,
			Nanos:                1028,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, true, uint64(1028)})

		injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
			capArgs = []interface{}{ctx, high, nanos}
			return nil
		}
		_, err = server.BoardDigitalInterruptTick(context.Background(), &pb.BoardDigitalInterruptTickRequest{
			BoardName:            "board1",
			DigitalInterruptName: "digital1",
			High:                 true,
			Nanos:                1028,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, true, uint64(1028)})
	})

	t.Run("CameraFrame", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			capName = name
			return nil, false
		}

		_, err := server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")
		test.That(t, capName, test.ShouldEqual, "camera1")

		injectCamera := &inject.Camera{}
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			return injectCamera, true
		}
		err1 := errors.New("whoops")
		injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return nil, nil, err1
		}
		_, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		var imgBuf bytes.Buffer
		test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

		var released bool
		injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return img, func() { released = true }, nil
		}

		resp, err := server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		released = false
		resp, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name:     "camera1",
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		released = false
		_, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name:     "camera1",
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, released, test.ShouldBeTrue)
	})

	t.Run("CameraRenderFrame", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			capName = name
			return nil, false
		}

		_, err := server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")
		test.That(t, capName, test.ShouldEqual, "camera1")

		injectCamera := &inject.Camera{}
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			return injectCamera, true
		}
		err1 := errors.New("whoops")
		injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return nil, nil, err1
		}
		_, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		var imgBuf bytes.Buffer
		test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

		var released bool
		injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return img, func() { released = true }, nil
		}

		resp, err := server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		released = false
		resp, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name:     "camera1",
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		released = false
		_, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name:     "camera1",
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, released, test.ShouldBeTrue)
	})

	t.Run("PointCloud", func(t *testing.T) {
		server, injectRobot := newServer()

		injectCamera := &inject.Camera{}
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			return injectCamera, true
		}
		err1 := errors.New("whoops")
		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, err1
		}
		_, err := server.PointCloud(context.Background(), &pb.PointCloudRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcA, nil
		}
		_, err = server.PointCloud(context.Background(), &pb.PointCloudRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ObjectPointClouds", func(t *testing.T) {
		server, injectRobot := newServer()

		injectCamera := &inject.Camera{}
		injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
			return injectCamera, true
		}
		err1 := errors.New("whoops")

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return nil, err1
		}
		_, err := server.ObjectPointClouds(context.Background(), &pb.ObjectPointCloudsRequest{
			Name:               "camera1",
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldEqual, err1)

		// request the two segments in the point cloud
		pcA := pointcloud.New()
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 4))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 5))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 6))
		test.That(t, err, test.ShouldBeNil)
		err = pcA.Set(pointcloud.NewBasicPoint(-5, -5, 4))
		test.That(t, err, test.ShouldBeNil)

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcA, nil
		}
		segs, err := server.ObjectPointClouds(context.Background(), &pb.ObjectPointCloudsRequest{
			Name:               "camera1",
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs.Frames), test.ShouldEqual, 2)
		test.That(t, segs.Centers[0].Z, test.ShouldEqual, 5.)
		test.That(t, segs.Centers[1].Z, test.ShouldEqual, 5.)
		test.That(t, segs.BoundingBoxes[0].Width, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[0].Length, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[0].Depth, test.ShouldEqual, 2)
		test.That(t, segs.BoundingBoxes[1].Width, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[1].Length, test.ShouldEqual, 0)
		test.That(t, segs.BoundingBoxes[1].Depth, test.ShouldEqual, 2)

		//empty pointcloud
		pcB := pointcloud.New()

		injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
			return pcB, nil
		}
		segs, err = server.ObjectPointClouds(context.Background(), &pb.ObjectPointCloudsRequest{
			Name:               "camera1",
			MinPointsInPlane:   100,
			MinPointsInSegment: 3,
			ClusteringRadius:   5.,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(segs.Frames), test.ShouldEqual, 0)
	})

	t.Run("Lidar", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.LidarByNameFunc = func(name string) (lidar.Lidar, bool) {
			capName = name
			return nil, false
		}

		_, err := server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no lidar")
		test.That(t, capName, test.ShouldEqual, "lidar1")

		err1 := errors.New("whoops")

		device := &inject.Lidar{}
		injectRobot.LidarByNameFunc = func(name string) (lidar.Lidar, bool) {
			return device, true
		}

		device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, err1
		}
		_, err = server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{"hello": true}, nil
		}
		infoResp, err := server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, infoResp.GetInfo().AsMap(), test.ShouldResemble, map[string]interface{}{"hello": true})

		device.StartFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.LidarStart(context.Background(), &pb.LidarStartRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StartFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.LidarStart(context.Background(), &pb.LidarStartRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.StopFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.LidarStop(context.Background(), &pb.LidarStopRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StopFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.LidarStop(context.Background(), &pb.LidarStopRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			return nil, err1
		}
		_, err = server.LidarScan(context.Background(), &pb.LidarScanRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		var capOptions lidar.ScanOptions
		ms := lidar.Measurements{lidar.NewMeasurement(0, 1), lidar.NewMeasurement(1, 2)}
		device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			capOptions = options
			return ms, nil
		}
		scanResp, err := server.LidarScan(context.Background(), &pb.LidarScanRequest{
			Name:  "lidar1",
			Count: 4, NoFilter: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, client.MeasurementsFromProto(scanResp.GetMeasurements()), test.ShouldResemble, ms)
		test.That(t, capOptions, test.ShouldResemble, lidar.ScanOptions{Count: 4, NoFilter: true})

		device.RangeFunc = func(ctx context.Context) (float64, error) {
			return 0, err1
		}
		_, err = server.LidarRange(context.Background(), &pb.LidarRangeRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.RangeFunc = func(ctx context.Context) (float64, error) {
			return 5, nil
		}
		rangeResp, err := server.LidarRange(context.Background(), &pb.LidarRangeRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rangeResp.GetRange(), test.ShouldEqual, 5)

		device.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{}, err1
		}
		_, err = server.LidarBounds(context.Background(), &pb.LidarBoundsRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{4, 5}, nil
		}
		boundsResp, err := server.LidarBounds(context.Background(), &pb.LidarBoundsRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, boundsResp.GetX(), test.ShouldEqual, 4)
		test.That(t, boundsResp.GetY(), test.ShouldEqual, 5)

		device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.LidarAngularResolution(context.Background(), &pb.LidarAngularResolutionRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return 6.2, nil
		}
		angResp, err := server.LidarAngularResolution(context.Background(), &pb.LidarAngularResolutionRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, angResp.GetAngularResolution(), test.ShouldEqual, 6.2)
	})

	t.Run("Sensor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			capName = name
			return nil, false
		}

		_, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldEqual, "compass1")

		err1 := errors.New("whoops")

		device := &inject.Compass{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return device, true
		}

		device.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return nil, err1
		}
		_, err = server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{1.2, 2.3}, nil
		}
		resp, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		readings := make([]interface{}, 0, len(resp.Readings))
		for _, r := range resp.Readings {
			readings = append(readings, r.AsInterface())
		}
		test.That(t, readings, test.ShouldResemble, []interface{}{1.2, 2.3})
	})

	t.Run("Compass", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			capName = name
			return nil, false
		}

		_, err := server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldEqual, "compass1")

		err1 := errors.New("whoops")

		device := &inject.Compass{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return device, true
		}

		device.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 1.2, nil
		}
		resp, err := server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Heading, test.ShouldResemble, 1.2)

		device.StartCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StartCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.StopCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StopCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not relative")

		relDevice := &inject.RelativeCompass{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return relDevice, true
		}

		relDevice.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 1.2, nil
		}
		resp, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Heading, test.ShouldResemble, 1.2)

		relDevice.StartCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.StartCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		relDevice.StopCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.StopCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		relDevice.MarkFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.MarkFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("IMU", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			capName = name
			return nil, false
		}

		_, err := server.IMUAngularVelocity(context.Background(), &pb.IMUAngularVelocityRequest{
			Name: "imu1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldEqual, "imu1")

		err1 := errors.New("whoops")

		device := &inject.IMU{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return device, true
		}

		device.AngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
			return spatialmath.AngularVelocity{}, err1
		}
		_, err = server.IMUAngularVelocity(context.Background(), &pb.IMUAngularVelocityRequest{
			Name: "imu1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.AngularVelocityFunc = func(ctx context.Context) (spatialmath.AngularVelocity, error) {
			return spatialmath.AngularVelocity{1, 2, 3}, nil
		}
		velResp, err := server.IMUAngularVelocity(context.Background(), &pb.IMUAngularVelocityRequest{
			Name: "imu1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, velResp.AngularVelocity, test.ShouldResemble, &pb.AngularVelocity{X: 1, Y: 2, Z: 3})

		device.OrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
			return nil, err1
		}
		_, err = server.IMUOrientation(context.Background(), &pb.IMUOrientationRequest{
			Name: "imu1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.OrientationFunc = func(ctx context.Context) (spatialmath.Orientation, error) {
			return &spatialmath.EulerAngles{1, 2, 3}, nil
		}
		orientResp, err := server.IMUOrientation(context.Background(), &pb.IMUOrientationRequest{
			Name: "imu1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, orientResp.Orientation, test.ShouldResemble, &pb.EulerAngles{Roll: 1, Pitch: 2, Yaw: 3})
	})

	t.Run("ServoMove", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ServoMove(context.Background(), &pb.ServoMoveRequest{
			Name: "servo1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")
		test.That(t, capName, test.ShouldEqual, "servo1")

		injectServo := &inject.Servo{}
		injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
			return injectServo, true
		}

		var capAngle uint8
		err1 := errors.New("whoops")
		injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
			capAngle = angle
			return err1
		}
		_, err = server.ServoMove(context.Background(), &pb.ServoMoveRequest{
			Name:     "servo1",
			AngleDeg: 5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAngle, test.ShouldEqual, 5)

		injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
			capAngle = angle
			return nil
		}
		_, err = server.ServoMove(context.Background(), &pb.ServoMoveRequest{
			Name:     "servo1",
			AngleDeg: 5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capAngle, test.ShouldEqual, 5)
	})

	t.Run("ServoCurrent", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ServoCurrent(context.Background(), &pb.ServoCurrentRequest{
			Name: "servo1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")
		test.That(t, capName, test.ShouldEqual, "servo1")

		injectServo := &inject.Servo{}
		injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
			return injectServo, true
		}

		var capCtx context.Context
		err1 := errors.New("whoops")
		injectServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
			capCtx = ctx
			return 0, err1
		}
		ctx := context.Background()
		_, err = server.ServoCurrent(context.Background(), &pb.ServoCurrentRequest{
			Name: "servo1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
			capCtx = ctx
			return 8, nil
		}
		currentResp, err := server.ServoCurrent(context.Background(), &pb.ServoCurrentRequest{
			Name: "servo1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capCtx, test.ShouldEqual, ctx)
		test.That(t, currentResp.AngleDeg, test.ShouldEqual, 8)
	})

	t.Run("Motor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			capName = name
			return nil, false
		}

		injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			capName = name
			return nil, false
		}

		_, err := server.MotorGo(context.Background(), &pb.MotorGoRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")
		test.That(t, capName, test.ShouldEqual, "motor1")

		injectMotor := &inject.Motor{}
		injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
			return injectMotor, true
		}

		var capArgs []interface{}
		err1 := errors.New("whoops")
		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			capArgs = []interface{}{d, powerPct}
			return err1
		}
		_, err = server.MotorGo(context.Background(), &pb.MotorGoRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, float32(0)})

		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			capArgs = []interface{}{d, powerPct}
			return nil
		}
		_, err = server.MotorGo(context.Background(), &pb.MotorGoRequest{
			Name:      "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD,
			PowerPct:  2,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(2)})

		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			return errors.New("no")
		}
		injectMotor.GoForFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
			capArgs = []interface{}{d, rpm, revolutions}
			return err1
		}
		_, err = server.MotorGoFor(context.Background(), &pb.MotorGoForRequest{
			Name:        "motor1",
			Direction:   pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Rpm:         2.3,
			Revolutions: 4.5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})

		injectMotor.GoForFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
			capArgs = []interface{}{d, rpm, revolutions}
			return nil
		}
		_, err = server.MotorGoFor(context.Background(), &pb.MotorGoForRequest{
			Name:        "motor1",
			Direction:   pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Rpm:         2.3,
			Revolutions: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})

		injectMotor.GoToFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
			capArgs = []interface{}{rpm, revolutions}
			return nil
		}
		_, err = server.MotorGoTo(context.Background(), &pb.MotorGoToRequest{
			Name:     "motor1",
			Rpm:      2.3,
			Position: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{2.3, 4.5})

		injectMotor.GoTillStopFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
			capArgs = []interface{}{d, rpm, stopFunc}
			return nil
		}
		_, err = server.MotorGoTillStop(context.Background(), &pb.MotorGoTillStopRequest{
			Name:      "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Rpm:       2.3,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, (func(context.Context) bool)(nil)})

		injectMotor.ZeroFunc = func(ctx context.Context, offset float64) error {
			capArgs = []interface{}{offset}
			return nil
		}
		_, err = server.MotorZero(context.Background(), &pb.MotorZeroRequest{
			Name:   "motor1",
			Offset: 5.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{5.5})

		ctx := context.Background()

		injectMotor.PowerFunc = func(ctx context.Context, powerPct float32) error {
			capArgs = []interface{}{ctx, powerPct}
			return err1
		}
		_, err = server.MotorPower(ctx, &pb.MotorPowerRequest{
			Name:     "motor1",
			PowerPct: 1.23,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, float32(1.23)})

		injectMotor.PowerFunc = func(ctx context.Context, powerPct float32) error {
			capArgs = []interface{}{ctx, powerPct}
			return nil
		}
		_, err = server.MotorPower(ctx, &pb.MotorPowerRequest{
			Name:     "motor1",
			PowerPct: 1.23,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, float32(1.23)})

		injectMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			capArgs = []interface{}{ctx}
			return math.NaN(), err1
		}
		_, err = server.MotorPosition(ctx, &pb.MotorPositionRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectMotor.PositionFunc = func(ctx context.Context) (float64, error) {
			capArgs = []interface{}{ctx}
			return 1.23, nil
		}
		posResp, err := server.MotorPosition(ctx, &pb.MotorPositionRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})
		test.That(t, posResp.Position, test.ShouldEqual, 1.23)

		injectMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			capArgs = []interface{}{ctx}
			return false, err1
		}
		_, err = server.MotorPositionSupported(ctx, &pb.MotorPositionSupportedRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
			capArgs = []interface{}{ctx}
			return true, nil
		}
		posSupportedResp, err := server.MotorPositionSupported(ctx, &pb.MotorPositionSupportedRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})
		test.That(t, posSupportedResp.Supported, test.ShouldBeTrue)

		injectMotor.OffFunc = func(ctx context.Context) error {
			capArgs = []interface{}{ctx}
			return err1
		}
		_, err = server.MotorOff(ctx, &pb.MotorOffRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectMotor.OffFunc = func(ctx context.Context) error {
			capArgs = []interface{}{ctx}
			return nil
		}
		_, err = server.MotorOff(ctx, &pb.MotorOffRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
			capArgs = []interface{}{ctx}
			return false, err1
		}
		_, err = server.MotorIsOn(ctx, &pb.MotorIsOnRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})

		injectMotor.IsOnFunc = func(ctx context.Context) (bool, error) {
			capArgs = []interface{}{ctx}
			return true, nil
		}
		isOnResp, err := server.MotorIsOn(ctx, &pb.MotorIsOnRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx})
		test.That(t, isOnResp.IsOn, test.ShouldBeTrue)
	})

	t.Run("Input", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
			capName = name
			return nil, false
		}

		_, err := server.InputControllerControls(context.Background(), &pb.InputControllerControlsRequest{
			Controller: "inputController1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")
		test.That(t, capName, test.ShouldEqual, "inputController1")

		err1 := errors.New("whoops")

		device := &inject.InputController{}
		injectRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
			if name == "inputController1" {
				return device, true
			}

			return nil, false
		}

		device.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
			return nil, err1
		}
		_, err = server.InputControllerControls(context.Background(), &pb.InputControllerControlsRequest{
			Controller: "inputController1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		device.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
			eventsOut := make(map[input.Control]input.Event)
			eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
			eventsOut[input.ButtonStart] = input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonStart, Value: 1.0}
			return eventsOut, nil
		}
		device.RegisterControlCallbackFunc = func(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
			outEvent := input.Event{Time: time.Now(), Event: triggers[0], Control: input.ButtonStart, Value: 0.0}
			ctrlFunc(ctx, outEvent)
			return nil
		}
		device.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
			return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
		}

		resp, err := server.InputControllerControls(context.Background(), &pb.InputControllerControlsRequest{
			Controller: "inputController1",
		})

		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Controls, test.ShouldResemble, []string{"AbsoluteX", "ButtonStart"})

		startTime := time.Now()
		resp2, err := server.InputControllerLastEvents(context.Background(), &pb.InputControllerLastEventsRequest{
			Controller: "inputController1",
		})

		test.That(t, err, test.ShouldBeNil)

		var absEv, buttonEv *pb.InputControllerEvent
		if resp2.Events[0].Control == "AbsoluteX" {
			absEv = resp2.Events[0]
			buttonEv = resp2.Events[1]
		} else {
			absEv = resp2.Events[1]
			buttonEv = resp2.Events[0]
		}

		test.That(t, absEv.Event, test.ShouldEqual, input.PositionChangeAbs)
		test.That(t, absEv.Control, test.ShouldEqual, input.AbsoluteX)
		test.That(t, absEv.Value, test.ShouldEqual, 0.7)
		test.That(t, absEv.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, absEv.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		test.That(t, buttonEv.Event, test.ShouldEqual, input.ButtonPress)
		test.That(t, buttonEv.Control, test.ShouldEqual, input.ButtonStart)
		test.That(t, buttonEv.Value, test.ShouldEqual, 1)
		test.That(t, buttonEv.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, buttonEv.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.InputControllerEvent, 1024)
		streamServer := &robotServiceInputControllerEventStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}

		eventReqList := &pb.InputControllerEventStreamRequest{
			Controller: "inputController2",
			Events: []*pb.InputControllerEventStreamRequest_Events{

				{
					Control: string(input.ButtonStart),
					Events: []string{
						string(input.ButtonRelease),
					},
				},
			},
		}

		err = server.InputControllerEventStream(eventReqList, streamServer)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no input controller")

		relayFunc := func(ctx context.Context, event input.Event) {
			messageCh <- &pb.InputControllerEvent{
				Time:    timestamppb.New(event.Time),
				Event:   string(event.Event),
				Control: string(event.Control),
				Value:   event.Value,
			}
		}

		err = device.RegisterControlCallback(cancelCtx, input.ButtonStart, []input.EventType{input.ButtonRelease}, relayFunc)
		test.That(t, err, test.ShouldBeNil)

		streamServer.fail = true

		eventReqList.Controller = "inputController1"

		err = server.InputControllerEventStream(eventReqList, streamServer)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")

		var streamErr error
		done := make(chan struct{})
		streamServer.fail = false
		go func() {
			streamErr = server.InputControllerEventStream(eventReqList, streamServer)
			close(done)
		}()

		resp3 := <-messageCh
		test.That(t, resp3.Control, test.ShouldEqual, string(input.ButtonStart))
		test.That(t, resp3.Event, test.ShouldEqual, input.ButtonRelease)
		test.That(t, resp3.Value, test.ShouldEqual, 0)
		test.That(t, resp3.Time.AsTime().After(startTime), test.ShouldBeTrue)
		test.That(t, resp3.Time.AsTime().Before(time.Now()), test.ShouldBeTrue)

		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)

	})

	t.Run("ForceMatrixMatrix", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			capName = name
			return nil, false
		}

		_, err := server.ForceMatrixMatrix(context.Background(), &pb.ForceMatrixMatrixRequest{
			Name: "fsm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no force matrix")
		test.That(t, capName, test.ShouldEqual, "fsm1")

		var capMatrix [][]int
		injectFsm := &inject.ForceMatrix{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return injectFsm, true
		}
		expectedMatrix := make([][]int, 4)
		for i := 0; i < len(expectedMatrix); i++ {
			expectedMatrix[i] = []int{1, 2, 3, 4}
		}
		injectFsm.MatrixFunc = func(ctx context.Context) ([][]int, error) {
			capMatrix = expectedMatrix
			return expectedMatrix, nil
		}
		_, err = server.ForceMatrixMatrix(context.Background(), &pb.ForceMatrixMatrixRequest{
			Name: "fsm1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capMatrix, test.ShouldResemble, expectedMatrix)
	})

	t.Run("ForceMatrixSlipDetection", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			capName = name
			return nil, false
		}
		_, err := server.ForceMatrixSlipDetection(context.Background(), &pb.ForceMatrixSlipDetectionRequest{
			Name: "fsm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, capName, test.ShouldEqual, "fsm1")

		injectFsm := &inject.ForceMatrix{}
		injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
			return injectFsm, true
		}
		injectFsm.IsSlippingFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		resp, err := server.ForceMatrixSlipDetection(context.Background(), &pb.ForceMatrixSlipDetectionRequest{
			Name: "fsm1",
		})
		test.That(t, resp.IsSlipping, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

	})

}

type robotServiceInputControllerEventStreamServer struct {
	grpc.ServerStream // not set
	ctx               context.Context
	messageCh         chan<- *pb.InputControllerEvent
	fail              bool
}

func (x *robotServiceInputControllerEventStreamServer) Context() context.Context {
	return x.ctx
}

func (x *robotServiceInputControllerEventStreamServer) Send(m *pb.InputControllerEvent) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}

type robotServiceStatusStreamServer struct {
	grpc.ServerStream // not set
	ctx               context.Context
	messageCh         chan<- *pb.StatusStreamResponse
	fail              bool
}

func (x *robotServiceStatusStreamServer) Context() context.Context {
	return x.ctx
}

func (x *robotServiceStatusStreamServer) Send(m *pb.StatusStreamResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}
