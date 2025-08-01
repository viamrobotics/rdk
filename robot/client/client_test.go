package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"go.uber.org/zap/zapcore"
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
	"go.viam.com/utils/rpc"
	gotestutils "go.viam.com/utils/testutils"
	"gonum.org/v1/gonum/num/quat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
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

var pose1 = spatialmath.NewZeroPose()

type mockRPCSubtypesUnimplemented struct {
	pb.UnimplementedRobotServiceServer
	ResourceNamesFunc func(*pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error)
}

func (ms *mockRPCSubtypesUnimplemented) ResourceNames(
	ctx context.Context, req *pb.ResourceNamesRequest,
) (*pb.ResourceNamesResponse, error) {
	return ms.ResourceNamesFunc(req)
}

func (ms *mockRPCSubtypesUnimplemented) GetMachineStatus(
	ctx context.Context, req *pb.GetMachineStatusRequest,
) (*pb.GetMachineStatusResponse, error) {
	return &pb.GetMachineStatusResponse{State: pb.GetMachineStatusResponse_STATE_RUNNING}, nil
}

type mockRPCSubtypesImplemented struct {
	mockRPCSubtypesUnimplemented
	ResourceNamesFunc func(*pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error)
}

func (ms *mockRPCSubtypesImplemented) ResourceRPCSubtypes(
	ctx context.Context, _ *pb.ResourceRPCSubtypesRequest,
) (*pb.ResourceRPCSubtypesResponse, error) {
	return &pb.ResourceRPCSubtypesResponse{}, nil
}

func (ms *mockRPCSubtypesImplemented) ResourceNames(
	ctx context.Context, req *pb.ResourceNamesRequest,
) (*pb.ResourceNamesResponse, error) {
	return ms.ResourceNamesFunc(req)
}

func (ms *mockRPCSubtypesImplemented) GetMachineStatus(
	ctx context.Context, req *pb.GetMachineStatusRequest,
) (*pb.GetMachineStatusResponse, error) {
	return &pb.GetMachineStatusResponse{State: pb.GetMachineStatusResponse_STATE_RUNNING}, nil
}

var resourceFunc1 = func(*pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error) {
	board1 := board.Named("board1")
	rNames := []*commonpb.ResourceName{
		{
			Namespace: string(board1.API.Type.Namespace),
			Type:      board1.API.Type.Name,
			Subtype:   board1.API.SubtypeName,
			Name:      board1.Name,
		},
	}
	return &pb.ResourceNamesResponse{Resources: rNames}, nil
}

var resourceFunc2 = func(*pb.ResourceNamesRequest) (*pb.ResourceNamesResponse, error) {
	board1 := board.Named("board1")
	board2 := board.Named("board2")
	rNames := []*commonpb.ResourceName{
		{
			Namespace: string(board1.API.Type.Namespace),
			Type:      board1.API.Type.Name,
			Subtype:   board1.API.SubtypeName,
			Name:      board1.Name,
		},
		{
			Namespace: string(board2.API.Type.Namespace),
			Type:      board2.API.Type.Name,
			Subtype:   board2.API.SubtypeName,
			Name:      board2.Name,
		},
	}
	return &pb.ResourceNamesResponse{Resources: rNames}, nil
}

func makeRPCServer(logger logging.Logger, option rpc.ServerOption) (rpc.Server, net.Listener, error) {
	err := errors.New("failed to make rpc server")
	var addr string
	var listener net.Listener
	var server rpc.Server

	for i := 0; i < 10; i++ {
		port, err := utils.TryReserveRandomPort()
		if err != nil {
			continue
		}

		addr = fmt.Sprint("localhost:", port)
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			continue
		}

		server, err = rpc.NewServer(logger, option)
		if err != nil {
			continue
		}
		return server, listener, nil
	}
	return nil, nil, err
}

func TestUnimplementedRPCSubtypes(t *testing.T) {
	var client1 *RobotClient // test implemented
	var client2 *RobotClient // test unimplemented
	ctx1, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	ctx2, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	logger1 := logging.NewTestLogger(t)
	logger2 := logging.NewTestLogger(t)

	rpcServer1, listener1, err := makeRPCServer(logger1, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	rpcServer2, listener2, err := makeRPCServer(logger2, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, rpcServer2.Stop(), test.ShouldBeNil)
	}()
	defer func() {
		test.That(t, rpcServer1.Stop(), test.ShouldBeNil)
	}()

	implementedService := mockRPCSubtypesImplemented{
		ResourceNamesFunc: resourceFunc1,
	}

	unimplementedService := mockRPCSubtypesUnimplemented{
		ResourceNamesFunc: resourceFunc1,
	}

	err = rpcServer1.RegisterServiceServer(
		ctx1,
		&pb.RobotService_ServiceDesc,
		&implementedService,
		pb.RegisterRobotServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	err = rpcServer2.RegisterServiceServer(
		ctx2,
		&pb.RobotService_ServiceDesc,
		&unimplementedService,
		pb.RegisterRobotServiceHandlerFromEndpoint)
	test.That(t, err, test.ShouldBeNil)

	go func() {
		test.That(t, rpcServer1.Serve(listener1), test.ShouldBeNil)
	}()
	go func() {
		test.That(t, rpcServer2.Serve(listener2), test.ShouldBeNil)
	}()

	client1, err = New(
		ctx1,
		listener1.Addr().String(),
		logger1,
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client1.Close(ctx1), test.ShouldBeNil)
	}()
	test.That(t, client1.Connected(), test.ShouldBeTrue)
	test.That(t, client1.rpcSubtypesUnimplemented, test.ShouldBeFalse)

	client2, err = New(
		ctx2,
		listener2.Addr().String(),
		logger2,
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client2.Close(ctx2), test.ShouldBeNil)
	}()
	test.That(t, client2.Connected(), test.ShouldBeTrue)
	test.That(t, client2.rpcSubtypesUnimplemented, test.ShouldBeTrue)

	// verify that the unimplemented check does not affect calls to ResourceNames
	test.That(t, len(client2.ResourceNames()), test.ShouldEqual, 1)
	_, err = client2.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)

	// still unimplemented, but with two resources
	unimplementedService.ResourceNamesFunc = resourceFunc2
	err = client2.Refresh(ctx2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(client2.ResourceNames()), test.ShouldEqual, 2)
	_, err = client2.ResourceByName(board.Named("board2"))
	test.That(t, err, test.ShouldBeNil)
}

func TestStatusClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}
	injectRobot2 := &inject.Robot{
		FrameSystemConfigFunc: frameSystemConfigFunc,
		ResourceNamesFunc:     resourcesFunc,
		ResourceRPCAPIsFunc:   func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	injectBoard := &inject.Board{}

	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

	injectCamera.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		return imgBuf.Bytes(), camera.ImageMetadata{MimeType: rutils.MimeTypePNG}, nil
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
	injectGripper.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return nil, nil
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

	err = arm1.MoveToJointPositions(context.Background(), []referenceframe.Input{}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	board1, err := board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldNotBeNil)
	pin, err := board1.GPIOPinByName("pin")
	test.That(t, err, test.ShouldBeNil)
	_, err = pin.Get(context.Background(), nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	camera1, err := camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	imgBytes, metadata, err := camera1.Image(context.Background(), rutils.MimeTypeJPEG, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, imgBytes, test.ShouldBeNil)
	test.That(t, metadata, test.ShouldResemble, camera.ImageMetadata{})

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

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), []referenceframe.Input{}, nil)
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

	frame, err := camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypeRawRGBA, nil, camera1)
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion

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
	logger := logging.NewTestLogger(t)

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
		injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
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
		testutils.VerifySameElements(t, arm.NamesFromRobot(client), testutils.ExtractNames(armNames...))
		testutils.VerifySameElements(t, base.NamesFromRobot(client), testutils.ExtractNames(baseNames...))

		testutils.VerifySameResourceNames(t, client.ResourceNames(), finalResources)

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
		testutils.VerifySameElements(t, arm.NamesFromRobot(client), testutils.ExtractNames(armNames...))
		testutils.VerifySameElements(t, base.NamesFromRobot(client), testutils.ExtractNames(baseNames...))

		testutils.VerifySameResourceNames(t, client.ResourceNames(), emptyResources)

		mu.Lock()
		injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
		injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }
		mu.Unlock()
		test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

		armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
		baseNames = []resource.Name{base.Named("base2"), base.Named("base3")}

		test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
		testutils.VerifySameElements(t, arm.NamesFromRobot(client), testutils.ExtractNames(armNames...))
		testutils.VerifySameElements(t, base.NamesFromRobot(client), testutils.ExtractNames(baseNames...))

		testutils.VerifySameResourceNames(t, client.ResourceNames(), finalResources)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestClientDisconnect(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1")}
	}
	injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	go gServer.Serve(listener)

	start := time.Now()

	test.That(t, err, test.ShouldBeNil)

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(dur),
		WithReconnectEvery(2*dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(t, client.Connected(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 1)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)

	gServer.Stop()
	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeFalse)
	timeSinceStart := time.Since(start)
	test.That(t, timeSinceStart, test.ShouldBeBetweenOrEqual, dur, 4*dur)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())
}

func TestClientUnaryDisconnectHandler(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	var unaryStatusCallReceived bool
	justOneUnaryStatusCall := grpc.ChainUnaryInterceptor(
		func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			// Allow a single GetMachineStatus through; return `io.ErrClosedPipe`
			// after that.
			if strings.HasSuffix(info.FullMethod, "RobotService/GetMachineStatus") {
				if unaryStatusCallReceived {
					return nil, status.Error(codes.Unknown, io.ErrClosedPipe.Error())
				}
				unaryStatusCallReceived = true
			}
			return handler(ctx, req)
		},
	)
	gServer := grpc.NewServer(justOneUnaryStatusCall)

	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
	injectRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)

	never := -1 * time.Second
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(never),
		WithReconnectEvery(never),
	)
	test.That(t, err, test.ShouldBeNil)
	// Reset unaryStatusCallReceived to false, as `New` call above set it to
	// true.
	unaryStatusCallReceived = false

	t.Run("unary call to connected remote", func(t *testing.T) {
		client.connected.Store(false)
		_, err = client.MachineStatus(context.Background())
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, unaryStatusCallReceived, test.ShouldBeFalse)
		client.connected.Store(true)
	})

	t.Run("unary call to disconnected remote", func(t *testing.T) {
		_, err = client.MachineStatus(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("unary call to undetected disconnected remote", func(t *testing.T) {
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
		_, err = client.MachineStatus(context.Background())
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
	})

	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()
	gServer.Stop()
}

func TestClientStreamDisconnectHandler(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	var streamStatusCallReceived bool
	interceptStreamStatusCall := grpc.ChainStreamInterceptor(
		func(
			srv interface{},
			ss grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			if strings.HasSuffix(info.FullMethod, "RobotService/StreamStatus") {
				streamStatusCallReceived = true
			}
			return handler(srv, ss)
		},
	)

	gServer := grpc.NewServer(interceptStreamStatusCall)

	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
	injectRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)

	never := -1 * time.Second
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(never),
		WithReconnectEvery(never),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("stream call to disconnected remote", func(t *testing.T) {
		t.Helper()

		client.connected.Store(false)
		//nolint:staticcheck // the status API is deprecated
		_, err = client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, streamStatusCallReceived, test.ShouldBeFalse)
		client.connected.Store(true)
	})

	t.Run("stream call to connected remote", func(t *testing.T) {
		t.Helper()

		//nolint:staticcheck // the status API is deprecated
		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		ssc.Recv()
		test.That(t, streamStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("receive call from stream of disconnected remote", func(t *testing.T) {
		t.Helper()

		//nolint:staticcheck // the status API is deprecated
		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)

		client.connected.Store(false)
		_, err = ssc.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		client.connected.Store(true)
	})

	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()
	gServer.Stop()
}

type mockType struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

func TestClientReconnect(t *testing.T) {
	someAPI := resource.APINamespace("acme").WithComponentType(uuid.New().String())
	var called int64
	resource.RegisterAPI(
		someAPI,
		resource.APIRegistration[resource.Resource]{
			RPCClient: func(
				ctx context.Context,
				conn rpc.ClientConn,
				remoteName string,
				name resource.Name,
				logger logging.Logger,
			) (resource.Resource, error) {
				atomic.AddInt64(&called, 1)
				return &mockType{Named: name.AsNamed()}, nil
			},
		},
	)

	logger := logging.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	thing1Name := resource.NewName(someAPI, "thing1")
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1"), thing1Name}
	}
	injectRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}
	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc2, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{arm.Named("arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	gServer.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc2))

	go gServer.Serve(listener)

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithCheckConnectedEvery(dur),
		WithReconnectEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)
	_, err = client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)
	a, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = a.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)

	gServer.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())

	gServer2 := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot))
	gServer2.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc2))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener, err = net.Listen("tcp", listener.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	go gServer2.Serve(listener)
	defer gServer2.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = a.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
}

func TestClientRefreshNoReconfigure(t *testing.T) {
	someAPI := resource.APINamespace("acme").WithComponentType(uuid.New().String())
	var called int64
	resource.RegisterAPI(
		someAPI,
		resource.APIRegistration[resource.Resource]{
			RPCClient: func(
				ctx context.Context,
				conn rpc.ClientConn,
				remoteName string,
				name resource.Name,
				logger logging.Logger,
			) (resource.Resource, error) {
				atomic.AddInt64(&called, 1)
				return &mockType{Named: name.AsNamed()}, nil
			},
		},
	)

	logger := logging.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	thing1Name := resource.NewName(someAPI, "thing1")
	injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	var callCount int
	calledEnough := make(chan struct{})

	allow := make(chan struct{})
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		if callCount == 1 {
			<-allow
		}
		if callCount == 5 {
			close(calledEnough)
		}
		callCount++

		return []resource.Name{arm.Named("arm1"), thing1Name}
	}

	go gServer.Serve(listener)
	defer gServer.Stop()

	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	close(allow)
	<-calledEnough

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)

	_, err = client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
}

func TestClientDialerOption(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := New(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	client2, err := New(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = client1.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
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
	injectRobot.MachineStatusFunc = func(_ context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := logging.NewTestLogger(t)

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

func TestClientGetModelsFromModules(t *testing.T) {
	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return finalResources
	}
	injectRobot.MachineStatusFunc = func(_ context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}
	expectedModels := []resource.ModuleModel{
		{
			ModuleName:      "simple-module",
			API:             resource.NewAPI("rdk", "component", "generic"),
			Model:           resource.NewModel("acme", "demo", "mycounter"),
			FromLocalModule: false,
		},
		{
			ModuleName:      "simple-module2",
			API:             resource.NewAPI("rdk", "component", "generic"),
			Model:           resource.NewModel("acme", "demo", "mycounter"),
			FromLocalModule: true,
		},
	}
	injectRobot.GetModelsFromModulesFunc = func(context.Context) ([]resource.ModuleModel, error) {
		return expectedModels, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := logging.NewTestLogger(t)

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	resp, err := client.GetModelsFromModules(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 2)
	test.That(t, resp, test.ShouldResemble, expectedModels)
	for index, model := range resp {
		test.That(t, model.ModuleName, test.ShouldEqual, expectedModels[index].ModuleName)
		test.That(t, model.Model, test.ShouldResemble, expectedModels[index].Model)
		test.That(t, model.API, test.ShouldResemble, expectedModels[index].API)
		test.That(t, model.FromLocalModule, test.ShouldEqual, expectedModels[index].FromLocalModule)
	}

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func ensurePartsAreEqual(part, otherPart *referenceframe.FrameSystemPart) error {
	if part.FrameConfig.Name() != otherPart.FrameConfig.Name() {
		return fmt.Errorf("part had name %s while other part had name %s", part.FrameConfig.Name(), otherPart.FrameConfig.Name())
	}
	frameConfig := part.FrameConfig
	otherFrameConfig := otherPart.FrameConfig
	if frameConfig.Parent() != otherFrameConfig.Parent() {
		return fmt.Errorf("part had parent %s while other part had parent %s", frameConfig.Parent(), otherFrameConfig.Parent())
	}
	if !spatialmath.R3VectorAlmostEqual(frameConfig.Pose().Point(), otherFrameConfig.Pose().Point(), 1e-8) {
		return errors.New("translations of parts not equal")
	}

	orient := frameConfig.Pose().Orientation()
	otherOrient := otherFrameConfig.Pose().Orientation()

	switch {
	case orient == nil && otherOrient != nil:
		if !spatialmath.QuaternionAlmostEqual(otherOrient.Quaternion(), quat.Number{1, 0, 0, 0}, 1e-5) {
			return errors.New("orientations of parts not equal")
		}
	case otherOrient == nil:
		return errors.New("orientation not returned for other part")
	case !spatialmath.OrientationAlmostEqual(orient, otherOrient):
		return errors.New("orientations of parts not equal")
	}
	return nil
}

func TestClientConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer := grpc.NewServer()
	failingServer := grpc.NewServer()

	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	workingRobot := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	failingRobot := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	l1 := &referenceframe.LinkConfig{
		ID:          "frame1",
		Parent:      referenceframe.World,
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: o1Cfg,
		Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif1, err := l1.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	l2 := &referenceframe.LinkConfig{
		ID:          "frame2",
		Parent:      "frame1",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif2, err := l2.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	fsConfigs := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
		{
			FrameConfig: lif2,
		},
	}

	workingRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: fsConfigs}, nil
	}

	configErr := errors.New("failed to retrieve config")
	failingRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return nil, configErr
	}

	pb.RegisterRobotServiceServer(workingServer, server.New(workingRobot))
	pb.RegisterRobotServiceServer(failingServer, server.New(failingRobot))

	go workingServer.Serve(listener1)
	defer workingServer.Stop()

	ctx := context.Background()

	t.Run("Failing client due to cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err = New(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	workingFSClient, err := New(ctx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for working frame service", func(t *testing.T) {
		config, err := workingFSClient.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], config.Parts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], config.Parts[1])
		test.That(t, err, test.ShouldBeNil)
	})

	err = workingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for working frame service", func(t *testing.T) {
		workingDialedClient, err := New(ctx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		config, err := workingDialedClient.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], config.Parts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], config.Parts[1])
		test.That(t, err, test.ShouldBeNil)
		err = workingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	failingFSClient, err := New(ctx, listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for failing frame service", func(t *testing.T) {
		frameSystemParts, err := failingFSClient.FrameSystemConfig(ctx)
		test.That(t, frameSystemParts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	err = failingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for failing frame service with failing config", func(t *testing.T) {
		failingDialedClient, err := New(ctx, listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		parts, err := failingDialedClient.FrameSystemConfig(ctx)
		test.That(t, parts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestForeignResource(t *testing.T) {
	injectRobot := &inject.Robot{}

	desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	subtype1 := resource.APINamespace("acme").WithComponentType("huwat")
	subtype2 := resource.APINamespace("acme").WithComponentType("wat")
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

	respWithResources := []resource.Name{
		arm.Named("arm1"),
		resource.NewName(subtype1, "thing1"),
		resource.NewName(subtype2, "thing2"),
	}

	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return respWith }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return respWithResources }
	injectRobot.MachineStatusFunc = func(_ context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}
	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	reflection.Register(gServer)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := logging.NewTestLogger(t)

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	res1, err := client.ResourceByName(respWithResources[0])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1, test.ShouldImplement, (*arm.Arm)(nil))

	res2, err := client.ResourceByName(respWithResources[1])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldHaveSameTypeAs, (*rgrpc.ForeignResource)(nil))
	test.That(t, res2.(*rgrpc.ForeignResource).Name(), test.ShouldResemble, respWithResources[1])

	res3, err := client.ResourceByName(respWithResources[2])
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res3, test.ShouldHaveSameTypeAs, (*rgrpc.ForeignResource)(nil))
	test.That(t, res3.(*rgrpc.ForeignResource).Name(), test.ShouldResemble, respWithResources[2])

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestNewRobotClientRefresh(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
	injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
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

func TestClientStopAll(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	stopAllCalled := false
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   resourcesFunc,
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		StopAllFunc: func(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
			stopAllCalled = true
			return nil
		},
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	err = client.StopAll(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stopAllCalled, test.ShouldBeTrue)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestRemoteClientMatch(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return validResources },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot1.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{arm.Named("remote:arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// working
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener1.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)

	resource1, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.resourceClients[arm.Named("remote:arm1")], test.ShouldEqual, resource1)
	pos, err := resource1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(pos, pose1), test.ShouldBeTrue)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestRemoteClientDuplicate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote1:arm1"), arm.Named("remote2:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return validResources },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{
		arm.Named("remote1:arm1"): injectArm,
		arm.Named("remote2:arm1"): injectArm,
	})
	test.That(t, err, test.ShouldBeNil)
	gServer1.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc1))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// working
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener1.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)

	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(arm.Named("arm1")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientOperationIntercept(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(_ context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	ctx := context.Background()
	var fakeArgs interface{}
	fakeManager := operation.NewManager(logger)
	ctx, done := fakeManager.Create(ctx, "fake", fakeArgs)
	defer done()
	fakeOp := operation.Get(ctx)
	test.That(t, fakeOp, test.ShouldNotBeNil)

	client, err := New(ctx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	injectRobot.MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		meta, ok := metadata.FromIncomingContext(ctx)
		test.That(t, ok, test.ShouldBeTrue)
		receivedOpID, err := operation.GetOrCreateFromMetadata(meta)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedOpID.String(), test.ShouldEqual, fakeOp.ID.String())
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	resp, err := client.MachineStatus(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldNotBeNil)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestGetUnknownResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{arm.Named("myArm")} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	// grabbing known resource is fine
	myArm, err := client.ResourceByName(arm.Named("myArm"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myArm, test.ShouldNotBeNil)

	// grabbing unknown resource returns error
	_, err = client.ResourceByName(base.Named("notABase"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(base.Named("notABase")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestLoggingInterceptor(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	// A server with the logging interceptor looks for some values in the grpc request metadata and
	// will call unary functions with a modified context.
	gServer := grpc.NewServer(grpc.ChainUnaryInterceptor(logging.UnaryServerInterceptor))
	injectRobot := &inject.Robot{
		// Needed for client connect. Not important to the test.
		ResourceNamesFunc:   func() []resource.Name { return []resource.Name{arm.Named("myArm")} },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },

		// Hijack the `MachineStatusFunc` for testing the reception of debug metadata via the
		// logging/distributed tracing interceptor.
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			// If there is no debug information with the context, return no revision
			if !logging.IsDebugMode(ctx) && logging.GetName(ctx) == "" {
				return robot.MachineStatus{State: robot.StateRunning}, nil
			}

			// If there is debug information with `oliver` with the context, return a revision of `oliver`
			if logging.IsDebugMode(ctx) && logging.GetName(ctx) == "oliver" {
				return robot.MachineStatus{Config: config.Revision{Revision: "oliver"}, State: robot.StateRunning}, nil
			}

			return robot.MachineStatus{State: robot.StateRunning}, errors.New("shouldn't happen")
		},
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	// Clients by default have an interceptor that serializes context debug information as grpc
	// metadata.
	client, err := New(context.Background(), listener.Addr().String(), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	defer client.Close(context.Background())

	// The machine status call with no debug information on the context should return no resource statuses.
	status, err := client.MachineStatus(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.Config.Revision, test.ShouldEqual, "")

	// The machine status call with debug information of `oliver` should return one resource status.
	status, err = client.MachineStatus(logging.EnableDebugModeWithKey(context.Background(), "oliver"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.Config.Revision, test.ShouldEqual, "oliver")
}

func TestCloudMetadata(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectCloudMD := cloud.Metadata{
		LocationID:    "the-location",
		PrimaryOrgID:  "the-primary-org",
		MachineID:     "the-machine",
		MachinePartID: "the-robot-part",
	}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return nil },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		CloudMetadataFunc: func(ctx context.Context) (cloud.Metadata, error) {
			return injectCloudMD, nil
		},
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}
	// TODO(RSDK-882): will update this so that this is not necessary
	injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{}, nil
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	md, err := client.CloudMetadata(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, md, test.ShouldResemble, injectCloudMD)
}

func TestShutDown(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	shutdownCalled := false
	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return nil },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		ShutdownFunc: func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		},
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	err = client.Shutdown(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, shutdownCalled, test.ShouldBeTrue)
}

func TestCurrentInputs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	testAPI := resource.APINamespaceRDK.WithComponentType(arm.SubtypeName)
	testName := resource.NewName(testAPI, "arm1")
	testName2 := resource.NewName(testAPI, "arm2")

	expectedInputs := referenceframe.FrameSystemInputs{
		testName.ShortName():  []referenceframe.Input{{0}, {math.Pi}, {-math.Pi}, {0}, {math.Pi}, {-math.Pi}},
		testName2.ShortName(): []referenceframe.Input{{math.Pi}, {-math.Pi}, {0}, {math.Pi}, {-math.Pi}, {0}},
	}
	injectArm := &inject.Arm{
		JointPositionsFunc: func(ctx context.Context, extra map[string]any) ([]referenceframe.Input, error) {
			return expectedInputs[testName.ShortName()], nil
		},
		KinematicsFunc: func(ctx context.Context) (referenceframe.Model, error) {
			return referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/example_kinematics/ur5e.json"), "")
		},
	}
	injectArm2 := &inject.Arm{
		JointPositionsFunc: func(ctx context.Context, extra map[string]any) ([]referenceframe.Input, error) {
			return expectedInputs[testName2.ShortName()], nil
		},
		KinematicsFunc: func(ctx context.Context) (referenceframe.Model, error) {
			return referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/example_kinematics/xarm6_kinematics_test.json"), "")
		},
	}
	resourceNames := []resource.Name{testName, testName2}
	resources := map[resource.Name]arm.Arm{testName: injectArm, testName2: injectArm2}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:  func() []resource.Name { return resourceNames },
		ResourceByNameFunc: func(n resource.Name) (resource.Resource, error) { return resources[n], nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	armSvc, err := resource.NewAPIResourceCollection(arm.API, resources)
	test.That(t, err, test.ShouldBeNil)
	gServer.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc))
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	inputs, err := client.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(inputs), test.ShouldEqual, 2)
	test.That(t, inputs, test.ShouldResemble, expectedInputs)
}

func TestUnregisteredResourceByName(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	testAPI := resource.APINamespace("testNamespace").WithComponentType("encoder")
	testName := resource.NewName(testAPI, "encoder1")

	testAPI2 := resource.APINamespaceRDK.WithComponentType("fake")
	testName2 := resource.NewName(testAPI2, "fake")

	resourceList := []resource.Name{
		testName,
		testName2,
	}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return resourceList },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	// We should not error when trying to create a client for an unregistered
	// resource, regardless of whether or not it is in RDK namespace.
	for _, name := range resourceList {
		_, err = client.ResourceByName(name)
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestMachineStatus(t *testing.T) {
	for _, tc := range []struct {
		name                string
		injectMachineStatus robot.MachineStatus
		expBadStateCount    int
	}{
		{
			"no resources",
			robot.MachineStatus{
				Config:    config.Revision{Revision: "rev1"},
				Resources: []resource.Status{},
				State:     robot.StateRunning,
			},
			0,
		},
		{
			"resource with unknown status",
			robot.MachineStatus{
				Config: config.Revision{Revision: "rev1"},
				Resources: []resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("badArm"),
							Revision: "rev0",
						},
					},
				},
				State: robot.StateRunning,
			},
			2, // once for client.New call and once for MachineStatus call
		},
		{
			"resource with valid status",
			robot.MachineStatus{
				Config: config.Revision{Revision: "rev1"},
				Resources: []resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("goodArm"),
							State:    resource.NodeStateConfiguring,
							Revision: "rev1",
						},
					},
				},
				State: robot.StateRunning,
			},
			0,
		},
		{
			"resources with mixed valid and invalid statuses",
			robot.MachineStatus{
				Config: config.Revision{Revision: "rev1"},
				Resources: []resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("goodArm"),
							State:    resource.NodeStateConfiguring,
							Revision: "rev1",
						},
					},
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("badArm"),
							Revision: "rev0",
						},
					},
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("anotherBadArm"),
							Revision: "rev-1",
						},
					},
				},
				State: robot.StateRunning,
			},
			4, // twice for client.New call and twice for MachineStatus call
		},
		{
			"unhealthy status",
			robot.MachineStatus{
				Config: config.Revision{Revision: "rev1"},
				Resources: []resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("brokenArm"),
							State:    resource.NodeStateUnhealthy,
							Error:    errors.New("bad configuration"),
							Revision: "rev1",
						},
					},
				},
				State: robot.StateRunning,
			},
			0,
		},
		{
			"cloud metadata",
			robot.MachineStatus{
				Config: config.Revision{Revision: "rev1"},
				Resources: []resource.Status{
					{
						NodeStatus: resource.NodeStatus{
							Name:     arm.Named("arm1"),
							State:    resource.NodeStateReady,
							Revision: "rev1",
						},
						CloudMetadata: cloud.Metadata{
							MachinePartID: "123",
							MachineID:     "456",
							PrimaryOrgID:  "789",
							LocationID:    "abc",
						},
					},
				},
				State: robot.StateRunning,
			},
			0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := logging.NewObservedTestLogger(t)
			listener, err := net.Listen("tcp", "localhost:0")
			test.That(t, err, test.ShouldBeNil)
			gServer := grpc.NewServer()

			injectRobot := &inject.Robot{
				LoggerFunc:          func() logging.Logger { return logger },
				ResourceNamesFunc:   func() []resource.Name { return nil },
				ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
				MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
					return tc.injectMachineStatus, nil
				},
			}
			// TODO(RSDK-882): will update this so that this is not necessary
			injectRobot.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
				return &framesystem.Config{}, nil
			}
			pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

			go gServer.Serve(listener)
			defer gServer.Stop()

			client, err := New(context.Background(), listener.Addr().String(), logger)
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				test.That(t, client.Close(context.Background()), test.ShouldBeNil)
			}()

			mStatus, err := client.MachineStatus(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, mStatus, test.ShouldResemble, tc.injectMachineStatus)

			const badStateMsg = "received resource in an unspecified state"
			badStateCount := logs.FilterLevelExact(zapcore.ErrorLevel).FilterMessageSnippet(badStateMsg).Len()
			test.That(t, badStateCount, test.ShouldEqual, tc.expBadStateCount)
		})
	}
}

func TestVersion(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return nil },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
	}

	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	version := robot.VersionResponse{
		Platform:   "rdk",
		Version:    "dev-unknown",
		APIVersion: "?",
	}
	md, err := client.Version(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, md, test.ShouldResemble, version)
}

func TestListTunnels(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	expectedTTEs := []config.TrafficTunnelEndpoint{
		{
			Port:              9090,
			ConnectionTimeout: 20 * time.Second,
		},
		{
			Port:              27017,
			ConnectionTimeout: 40 * time.Millisecond,
		},
		{
			Port: 23654,
		},
	}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:   func() []resource.Name { return nil },
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		ListTunnelsFunc: func(ctx context.Context) ([]config.TrafficTunnelEndpoint, error) {
			return expectedTTEs, nil
		},
	}

	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	ttes, err := client.ListTunnels(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ttes, test.ShouldResemble, expectedTTEs)
}
