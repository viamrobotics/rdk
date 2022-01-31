package server_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	grpcserver "go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
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
		Boards: map[string]*commonpb.BoardStatus{
			"board1": {
				Analogs: map[string]*commonpb.AnalogStatus{
					"analog1": {},
				},
				DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
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
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			services := make(map[resource.Name]interface{})
			service, ok := services[name]
			return service, ok
		}
		_, err := server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("no framesystem service"))

		// set up the robot with something that is not a framesystem service
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			services := make(map[resource.Name]interface{})
			services[framesystem.Name] = nil
			service, ok := services[name]
			return service, ok
		}
		_, err = server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("service is not a framesystem.Service"))

		// set up the robot with the frame system
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			services := make(map[resource.Name]interface{})
			services[framesystem.Name] = fss
			service, ok := services[name]
			return service, ok
		}

		fssResp, err := server.FrameServiceConfig(context.Background(), &pb.FrameServiceConfigRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(fssResp.FrameSystemConfigs), test.ShouldEqual, len(fsConfigs))
		test.That(t, fssResp.FrameSystemConfigs[0].Name, test.ShouldEqual, fsConfigs[0].Name)
		test.That(t, fssResp.FrameSystemConfigs[0].FrameConfig.Parent, test.ShouldEqual, fsConfigs[0].FrameConfig.Parent)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.X,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.X,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Y,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.Y,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Z,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Translation.Z,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OX,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OX,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OY,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OY,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.OZ,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().OZ,
		)
		test.That(t,
			fssResp.FrameSystemConfigs[0].FrameConfig.Pose.Theta,
			test.ShouldAlmostEqual,
			fsConfigs[0].FrameConfig.Orientation.OrientationVectorDegrees().Theta,
		)
		t.Logf("the json frame should be empty:\n %v", fssResp.FrameSystemConfigs[0].ModelJson)
		_, err = referenceframe.ParseJSON(fssResp.FrameSystemConfigs[0].ModelJson, fssResp.FrameSystemConfigs[0].Name)
		test.That(t, err, test.ShouldBeError, referenceframe.ErrNoModelInformation)
	})

	t.Run("ObjectManipulation", func(t *testing.T) {
		server, injectRobot := newServer()

		// set up the robot without an objectmanipulation service
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			services := make(map[resource.Name]interface{})
			service, ok := services[name]
			return service, ok
		}
		_, err := server.ObjectManipulationServiceDoGrab(context.Background(), &pb.ObjectManipulationServiceDoGrabRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("no objectmanipulation service"))

		// set up the robot with something that is not an objectmanipulation service
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			services := make(map[resource.Name]interface{})
			services[objectmanipulation.Name] = nil
			service, ok := services[name]
			return service, ok
		}
		_, err = server.ObjectManipulationServiceDoGrab(context.Background(), &pb.ObjectManipulationServiceDoGrabRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("service is not a objectmanipulation service"))

		// pass on dograb error
		passedErr := errors.New("fake dograb error")
		omSvc := &inject.ObjectManipulationService{}
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
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
			go utils.TryClose(context.Background(), server)
			<-ctx.Done()
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)
	})

	t.Run("Sensor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName resource.Name
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			capName = name
			return nil, false
		}

		_, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldResemble, sensor.Named("compass1"))

		err1 := errors.New("whoops")

		device := &inject.Sensor{}
		injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
			return device, true
		}

		device.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return nil, err1
		}
		_, err = server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "sensor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{1.2, 2.3}, nil
		}
		resp, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "sensor1",
		})
		test.That(t, err, test.ShouldBeNil)
		readings := make([]interface{}, 0, len(resp.Readings))
		for _, r := range resp.Readings {
			readings = append(readings, r.AsInterface())
		}
		test.That(t, readings, test.ShouldResemble, []interface{}{1.2, 2.3})
	})
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
