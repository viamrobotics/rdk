package server_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/component/input"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	grpcserver "go.viam.com/core/grpc/server"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	servicepkg "go.viam.com/core/services"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/testutils/inject"

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
		Servos: map[string]*pb.ServoStatus{
			"servo1": {},
		},
		Motors: map[string]*pb.MotorStatus{
			"motor1": {},
		},
		InputControllers: map[string]*pb.InputControllerStatus{
			"inputController1": {},
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
					Translation: spatialmath.Translation{1, 2, 3},
					Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
				},
			},
			{
				Name: "frame2",
				FrameConfig: &config.Frame{
					Parent:      "frame1",
					Translation: spatialmath.Translation{1, 2, 3},
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
		t.Logf("the json frame should be empty:\n %v", fssResp.FrameSystemConfigs[0].ModelJson)
		modelFrame, err := referenceframe.ParseJSON(fssResp.FrameSystemConfigs[0].ModelJson, fssResp.FrameSystemConfigs[0].Name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, modelFrame, test.ShouldEqual, fsConfigs[0].ModelFrame)
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
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return err1
		}
		_, err = server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldNotBeNil)

		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return nil
		}
		resp, err := server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			MillisPerSec:   2.3,
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 2.3, false})
		test.That(t, resp.Success, test.ShouldBeTrue)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return err1
		}
		_, err = server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:     "base1",
			AngleDeg: 4.5,
		})
		test.That(t, err, test.ShouldNotBeNil)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return nil
		}
		spinResp, err := server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:       "base1",
			AngleDeg:   4.5,
			DegsPerSec: 20.3,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 20.3, false})
		test.That(t, spinResp.Success, test.ShouldBeTrue)

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
		injectMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
			capArgs = []interface{}{powerPct}
			return err1
		}
		_, err = server.MotorGo(context.Background(), &pb.MotorGoRequest{
			Name: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{float64(0)})

		injectMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
			capArgs = []interface{}{powerPct}
			return nil
		}
		_, err = server.MotorGo(context.Background(), &pb.MotorGoRequest{
			Name:     "motor1",
			PowerPct: 2,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{float64(2)})

		injectMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
			return errors.New("no")
		}
		injectMotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
			capArgs = []interface{}{rpm, revolutions}
			return err1
		}
		_, err = server.MotorGoFor(context.Background(), &pb.MotorGoForRequest{
			Name:        "motor1",
			Rpm:         -2.3,
			Revolutions: 4.5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{-2.3, 4.5})

		injectMotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
			capArgs = []interface{}{rpm, revolutions}
			return nil
		}
		_, err = server.MotorGoFor(context.Background(), &pb.MotorGoForRequest{
			Name:        "motor1",
			Rpm:         2.3,
			Revolutions: -4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{2.3, -4.5})

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

		injectMotor.GoTillStopFunc = func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
			capArgs = []interface{}{rpm, stopFunc}
			return nil
		}
		_, err = server.MotorGoTillStop(context.Background(), &pb.MotorGoTillStopRequest{
			Name: "motor1",
			Rpm:  -2.3,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{-2.3, (func(context.Context) bool)(nil)})

		injectMotor.SetToZeroPositionFunc = func(ctx context.Context, offset float64) error {
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

		injectMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
			capArgs = []interface{}{ctx, powerPct}
			return err1
		}
		_, err = server.MotorPower(ctx, &pb.MotorPowerRequest{
			Name:     "motor1",
			PowerPct: 1.23,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, float64(1.23)})

		injectMotor.SetPowerFunc = func(ctx context.Context, powerPct float64) error {
			capArgs = []interface{}{ctx, powerPct}
			return nil
		}
		_, err = server.MotorPower(ctx, &pb.MotorPowerRequest{
			Name:     "motor1",
			PowerPct: 1.23,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, float64(1.23)})

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
