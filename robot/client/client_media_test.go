//go:build !no_media

package client

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	basepb "go.viam.com/api/component/base/v1"
	boardpb "go.viam.com/api/component/board/v1"
	camerapb "go.viam.com/api/component/camera/v1"
	gripperpb "go.viam.com/api/component/gripper/v1"
	inputcontrollerpb "go.viam.com/api/component/inputcontroller/v1"
	motorpb "go.viam.com/api/component/motor/v1"
	sensorpb "go.viam.com/api/component/sensor/v1"
	servopb "go.viam.com/api/component/servo/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	gotestutils "go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

var emptyResources = []resource.Name{
	arm.Named("arm1"),
	base.Named("base1"),
	board.Named("board1"),
	board.Named("board3"),
	camera.Named("camera1"),
	gripper.Named("gripper1"),
}

var finalResources = []resource.Name{
	arm.Named("arm2"),
	arm.Named("arm3"),
	base.Named("base2"),
	base.Named("base3"),
	board.Named("board2"),
	board.Named("board3"),
	camera.Named("camera2"),
	camera.Named("camera3"),
	gripper.Named("gripper2"),
	gripper.Named("gripper3"),
	motor.Named("motor2"),
	motor.Named("motor3"),
	servo.Named("servo2"),
	servo.Named("servo3"),
}

func TestStatusClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	resourcesFunc := func() []resource.Name {
		return []resource.Name{
			arm.Named("arm1"),
			base.Named("base1"),
			board.Named("board1"),
			camera.Named("camera1"),
			gripper.Named("gripper1"),
			input.Named("inputController1"),
			motor.Named("motor1"),
			motor.Named("motor2"),
			sensor.Named("sensor1"),
			servo.Named("servo1"),
		}
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	frameSystemConfigFunc := func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	injectRobot1 := &inject.Robot{
		FrameSystemConfigFunc: frameSystemConfigFunc,
		ResourceNamesFunc:     resourcesFunc,
		ResourceRPCAPIsFunc:   func() []resource.RPCAPI { return nil },
	}
	injectRobot2 := &inject.Robot{
		FrameSystemConfigFunc: frameSystemConfigFunc,
		ResourceNamesFunc:     resourcesFunc,
		ResourceRPCAPIsFunc:   func() []resource.RPCAPI { return nil },
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
		return nil, errors.New("no status")
	}

	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

	var imageReleased bool
	var imageReleasedMu sync.Mutex
	injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			imageReleasedMu.Lock()
			imageReleased = true
			imageReleasedMu.Unlock()
			return img, func() {}, nil
		})), nil
	}

	injectInputDev := &inject.InputController{}
	injectInputDev.ControlsFunc = func(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}

	injectGripper := &inject.Gripper{}
	var gripperOpenCalled bool
	injectGripper.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		gripperOpenCalled = true
		return nil
	}
	var gripperGrabCalled bool
	injectGripper.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		gripperGrabCalled = true
		return true, nil
	}

	injectServo := &inject.Servo{}
	var capServoAngle uint32
	injectServo.MoveFunc = func(ctx context.Context, angle uint32, extra map[string]interface{}) error {
		capServoAngle = angle
		return nil
	}
	injectServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 5, nil
	}

	armSvc1, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc1))

	armSvc2, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{arm.Named("arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc2))

	baseSvc, err := resource.NewAPIResourceCollection(base.API, map[resource.Name]base.Base{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&basepb.BaseService_ServiceDesc, base.NewRPCServiceServer(baseSvc))

	baseSvc2, err := resource.NewAPIResourceCollection(base.API, map[resource.Name]base.Base{base.Named("base1"): &inject.Base{}})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&basepb.BaseService_ServiceDesc, base.NewRPCServiceServer(baseSvc2))

	boardSvc1, err := resource.NewAPIResourceCollection(board.API, map[resource.Name]board.Board{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&boardpb.BoardService_ServiceDesc, board.NewRPCServiceServer(boardSvc1))

	boardSvc2, err := resource.NewAPIResourceCollection(board.API, map[resource.Name]board.Board{board.Named("board1"): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&boardpb.BoardService_ServiceDesc, board.NewRPCServiceServer(boardSvc2))

	cameraSvc1, err := resource.NewAPIResourceCollection(camera.API, map[resource.Name]camera.Camera{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&camerapb.CameraService_ServiceDesc, camera.NewRPCServiceServer(cameraSvc1))

	cameraSvc2, err := resource.NewAPIResourceCollection(camera.API, map[resource.Name]camera.Camera{camera.Named("camera1"): injectCamera})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&camerapb.CameraService_ServiceDesc, camera.NewRPCServiceServer(cameraSvc2))

	gripperSvc1, err := resource.NewAPIResourceCollection(gripper.API, map[resource.Name]gripper.Gripper{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&gripperpb.GripperService_ServiceDesc, gripper.NewRPCServiceServer(gripperSvc1))

	gripperSvc2, err := resource.NewAPIResourceCollection(gripper.API,
		map[resource.Name]gripper.Gripper{gripper.Named("gripper1"): injectGripper})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&gripperpb.GripperService_ServiceDesc, gripper.NewRPCServiceServer(gripperSvc2))

	inputControllerSvc1, err := resource.NewAPIResourceCollection(input.API, map[resource.Name]input.Controller{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&inputcontrollerpb.InputControllerService_ServiceDesc, input.NewRPCServiceServer(inputControllerSvc1))

	inputControllerSvc2, err := resource.NewAPIResourceCollection(
		input.API, map[resource.Name]input.Controller{input.Named("inputController1"): injectInputDev})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&inputcontrollerpb.InputControllerService_ServiceDesc, input.NewRPCServiceServer(inputControllerSvc2))

	motorSvc, err := resource.NewAPIResourceCollection(motor.API, map[resource.Name]motor.Motor{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&motorpb.MotorService_ServiceDesc, motor.NewRPCServiceServer(motorSvc))

	motorSvc2, err := resource.NewAPIResourceCollection(motor.API,
		map[resource.Name]motor.Motor{motor.Named("motor1"): &inject.Motor{}, motor.Named("motor2"): &inject.Motor{}})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&motorpb.MotorService_ServiceDesc, motor.NewRPCServiceServer(motorSvc2))

	servoSvc, err := resource.NewAPIResourceCollection(servo.API, map[resource.Name]servo.Servo{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&servopb.ServoService_ServiceDesc, servo.NewRPCServiceServer(servoSvc))

	servoSvc2, err := resource.NewAPIResourceCollection(servo.API, map[resource.Name]servo.Servo{servo.Named("servo1"): injectServo})
	test.That(t, err, test.ShouldBeNil)
	gServer2.RegisterService(&servopb.ServoService_ServiceDesc, servo.NewRPCServiceServer(servoSvc2))

	sensorSvc, err := resource.NewAPIResourceCollection(sensor.API, map[resource.Name]sensor.Sensor{})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&sensorpb.SensorService_ServiceDesc, sensor.NewRPCServiceServer(sensorSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	// failing
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(cancelCtx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	o1 := &spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: 7}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	cfg := config.Config{
		Components: []resource.Config{
			{
				Name: "a",
				API:  arm.API,
				Frame: &referenceframe.LinkConfig{
					Parent:      "b",
					Translation: r3.Vector{X: 1, Y: 2, Z: 3},
					Orientation: o1Cfg,
				},
			},
			{
				Name: "b",
				API:  base.API,
			},
		},
	}
	injectRobot1.ConfigFunc = func() *config.Config {
		return &cfg
	}

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.FromRobot(client, "arm1")
	test.That(t, err, test.ShouldBeNil)
	_, err = arm1.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	_, err = arm1.JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	err = arm1.MoveToPosition(context.Background(), spatialmath.NewPoseFromPoint(r3.Vector{X: 1}), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	err = arm1.MoveToJointPositions(context.Background(), &armpb.JointPositions{Values: []float64{1}}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	board1, err := board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldNotBeNil)
	test.That(t, board1.ModelAttributes(), test.ShouldResemble, board.ModelAttributes{Remote: true})

	_, err = board1.Status(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	camera1, err := camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), camera1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	gripper1, err := gripper.FromRobot(client, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	err = gripper1.Open(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	_, err = gripper1.Grab(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	motor1, err := motor.FromRobot(client, "motor1")
	test.That(t, err, test.ShouldBeNil)
	err = motor1.SetPower(context.Background(), 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	err = motor1.GoFor(context.Background(), 0, 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	sensorDevice, err := sensor.FromRobot(client, "sensor1")
	test.That(t, err, test.ShouldBeNil)
	_, err = sensorDevice.Readings(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	servo1, err := servo.FromRobot(client, "servo1")
	test.That(t, err, test.ShouldBeNil)
	err = servo1.Move(context.Background(), 5, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	_, err = servo1.Position(context.Background(), nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	resource1, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = resource1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	_, err = resource1.(arm.Arm).JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	err = resource1.(arm.Arm).MoveToPosition(context.Background(), spatialmath.NewPoseFromPoint(r3.Vector{X: 1}), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), &armpb.JointPositions{Values: []float64{1}}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// working
	client, err = New(context.Background(), listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, func() { client.RemoteByName("remote1") }, test.ShouldPanic)

	arm1, err = arm.FromRobot(client, "arm1")
	test.That(t, err, test.ShouldBeNil)
	pos, err := arm1.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(pos, pose1), test.ShouldBeTrue)

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	_, err = board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)

	camera1, err = camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	ctx := gostream.WithMIMETypeHint(context.Background(), rutils.MimeTypeRawRGBA)
	frame, _, err := camera.ReadImage(ctx, camera1)
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
	imageReleasedMu.Lock()
	test.That(t, imageReleased, test.ShouldBeTrue)
	imageReleasedMu.Unlock()

	gripper1, err = gripper.FromRobot(client, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	err = gripper1.Open(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)

	inputDev, err := input.FromRobot(client, "inputController1")
	test.That(t, err, test.ShouldBeNil)
	controlList, err := inputDev.Controls(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})

	motor1, err = motor.FromRobot(client, "motor1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motor1, test.ShouldNotBeNil)

	motor2, err := motor.FromRobot(client, "motor2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motor2, test.ShouldNotBeNil)

	servo1, err = servo.FromRobot(client, "servo1")
	test.That(t, err, test.ShouldBeNil)
	err = servo1.Move(context.Background(), 4, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)

	currentVal, err := servo1.Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currentVal, test.ShouldEqual, 5)

	resource1, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	pos, err = resource1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(pos, pose1), test.ShouldBeTrue)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientRefresh(t *testing.T) {
	logger := golog.NewTestLogger(t)

	listener := gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}

	var mu sync.RWMutex
	dur := 100 * time.Millisecond

	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	go gServer.Serve(listener)
	defer func() {
		mu.Lock()
		gServer.Stop()
		mu.Unlock()
	}()
	t.Run("run with same reconnectTime and checkConnectedTime", func(t *testing.T) {
		calledEnough := make(chan struct{})
		var callCountAPIs int
		var callCountNames int

		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
			mu.Lock()
			defer mu.Unlock()
			callCountAPIs++
			if callCountAPIs == 6 {
				close(calledEnough)
			}
			return nil
		}
		injectRobot.ResourceNamesFunc = func() []resource.Name {
			mu.Lock()
			defer mu.Unlock()
			callCountNames++
			return emptyResources
		}
		mu.Unlock()

		start := time.Now()
		client, err := New(
			context.Background(),
			listener.Addr().String(),
			logger,
			WithRefreshEvery(dur),
			WithCheckConnectedEvery(dur),
			WithReconnectEvery(dur),
		)
		test.That(t, err, test.ShouldBeNil)
		// block here until ResourceNames is called 6 times
		<-calledEnough
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 5*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 10*dur)
		test.That(t, callCountAPIs, test.ShouldEqual, 6)
		test.That(t, callCountNames, test.ShouldEqual, 6)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("run with different reconnectTime and checkConnectedTime", func(t *testing.T) {
		calledEnough := make(chan struct{})
		var callCountAPIs int
		var callCountNames int

		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
			mu.Lock()
			defer mu.Unlock()
			callCountAPIs++
			if callCountAPIs == 7 {
				close(calledEnough)
			}
			return nil
		}
		injectRobot.ResourceNamesFunc = func() []resource.Name {
			mu.Lock()
			defer mu.Unlock()
			callCountNames++
			return emptyResources
		}
		mu.Unlock()

		start := time.Now()
		client, err := New(
			context.Background(),
			listener.Addr().String(),
			logger,
			WithRefreshEvery(dur),
			WithCheckConnectedEvery(dur*2),
			WithReconnectEvery(dur),
		)
		test.That(t, err, test.ShouldBeNil)
		// block here until ResourceNames is called 7 times
		<-calledEnough
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 7*dur)
		test.That(t, callCountAPIs, test.ShouldEqual, 7)
		test.That(t, callCountNames, test.ShouldEqual, 7)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("refresh tests", func(t *testing.T) {
		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }
		mu.Unlock()
		client, _ := New(
			context.Background(),
			listener.Addr().String(),
			logger,
		)

		armNames := []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
		baseNames := []resource.Name{base.Named("base2"), base.Named("base3")}

		test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(arm.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(armNames...)...),
		)
		test.That(t,
			utils.NewStringSet(base.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
		)

		test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
			finalResources...))

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)

		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return emptyResources }
		mu.Unlock()
		client, err := New(
			context.Background(),
			listener.Addr().String(),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		armNames = []resource.Name{arm.Named("arm1")}
		baseNames = []resource.Name{base.Named("base1")}

		test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(arm.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(armNames...)...),
		)
		test.That(t,
			utils.NewStringSet(base.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
		)

		test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
			emptyResources...))

		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }
		mu.Unlock()
		test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

		armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
		baseNames = []resource.Name{base.Named("base2"), base.Named("base3")}

		test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(arm.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(armNames...)...),
		)
		test.That(t,
			utils.NewStringSet(base.NamesFromRobot(client)...),
			test.ShouldResemble,
			utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
		)

		test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
			finalResources...))

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestClientResources(t *testing.T) {
	injectRobot := &inject.Robot{}

	desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	respWith := []resource.RPCAPI{
		{
			API:  resource.APINamespace("acme").WithComponentType("huwat"),
			Desc: desc1,
		},
		{
			API:  resource.APINamespace("acme").WithComponentType("wat"),
			Desc: desc2,
		},
	}

	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return respWith }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	go gServer.Serve(listener)

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	// no reflection
	resources, rpcAPIs, err := client.resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, finalResources)
	test.That(t, rpcAPIs, test.ShouldBeEmpty)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	gServer.Stop()

	// with reflection
	gServer = grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	reflection.Register(gServer)
	test.That(t, err, test.ShouldBeNil)
	listener, err = net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err = New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	resources, rpcAPIs, err = client.resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, finalResources)

	test.That(t, rpcAPIs, test.ShouldHaveLength, len(respWith))
	for idx, rpcType := range rpcAPIs {
		otherT := respWith[idx]
		test.That(t, rpcType.API, test.ShouldResemble, otherT.API)
		test.That(t, rpcType.Desc.AsProto(), test.ShouldResemble, otherT.Desc.AsProto())
	}

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDiscovery(t *testing.T) {
	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return finalResources
	}
	q := resource.DiscoveryQuery{movementsensor.Named("foo").API, resource.DefaultModelFamily.WithModel("something")}
	injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []resource.DiscoveryQuery) ([]resource.Discovery, error) {
		return []resource.Discovery{{
			Query:   q,
			Results: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}},
		}}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	resp, err := client.DiscoverComponents(context.Background(), []resource.DiscoveryQuery{q})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 1)
	test.That(t, resp[0].Query, test.ShouldResemble, q)
	test.That(t, resp[0].Results, test.ShouldResemble, map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}})

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestNewRobotClientRefresh(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	var callCount int

	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		callCount++
		return emptyResources
	}

	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	dur := -100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, client, test.ShouldNotBeNil)
	test.That(t, callCount, test.ShouldEqual, 1)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	callCount = 0
	dur = 0
	client, err = New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client, test.ShouldNotBeNil)
	test.That(t, callCount, test.ShouldEqual, 1)

	gServer.Stop()
	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeFalse)
	err = client.Refresh(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "not connected to remote robot")

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
