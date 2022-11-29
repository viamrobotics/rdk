package client

import (
	"bytes"
	"context"
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

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
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
	"go.viam.com/rdk/discovery"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
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
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	injectRobot2 := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
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

	armSvc1, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	armpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc1))

	armSvc2, err := subtype.New(map[resource.Name]interface{}{arm.Named("arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	armpb.RegisterArmServiceServer(gServer2, arm.NewServer(armSvc2))

	baseSvc, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	basepb.RegisterBaseServiceServer(gServer1, base.NewServer(baseSvc))

	baseSvc2, err := subtype.New(map[resource.Name]interface{}{base.Named("base1"): &inject.Base{}})
	test.That(t, err, test.ShouldBeNil)
	basepb.RegisterBaseServiceServer(gServer2, base.NewServer(baseSvc2))

	boardSvc1, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	boardpb.RegisterBoardServiceServer(gServer1, board.NewServer(boardSvc1))

	boardSvc2, err := subtype.New(map[resource.Name]interface{}{board.Named("board1"): injectBoard})
	test.That(t, err, test.ShouldBeNil)
	boardpb.RegisterBoardServiceServer(gServer2, board.NewServer(boardSvc2))

	cameraSvc1, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	camerapb.RegisterCameraServiceServer(gServer1, camera.NewServer(cameraSvc1))

	cameraSvc2, err := subtype.New(map[resource.Name]interface{}{camera.Named("camera1"): injectCamera})
	test.That(t, err, test.ShouldBeNil)
	camerapb.RegisterCameraServiceServer(gServer2, camera.NewServer(cameraSvc2))

	gripperSvc1, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	gripperpb.RegisterGripperServiceServer(gServer1, gripper.NewServer(gripperSvc1))

	gripperSvc2, err := subtype.New(map[resource.Name]interface{}{gripper.Named("gripper1"): injectGripper})
	test.That(t, err, test.ShouldBeNil)
	gripperpb.RegisterGripperServiceServer(gServer2, gripper.NewServer(gripperSvc2))

	inputControllerSvc1, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	inputcontrollerpb.RegisterInputControllerServiceServer(gServer1, input.NewServer(inputControllerSvc1))

	inputControllerSvc2, err := subtype.New(map[resource.Name]interface{}{input.Named("inputController1"): injectInputDev})
	test.That(t, err, test.ShouldBeNil)
	inputcontrollerpb.RegisterInputControllerServiceServer(gServer2, input.NewServer(inputControllerSvc2))

	motorSvc, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	motorpb.RegisterMotorServiceServer(gServer1, motor.NewServer(motorSvc))

	motorSvc2, err := subtype.New(
		map[resource.Name]interface{}{motor.Named("motor1"): &inject.Motor{}, motor.Named("motor2"): &inject.Motor{}},
	)
	test.That(t, err, test.ShouldBeNil)
	motorpb.RegisterMotorServiceServer(gServer2, motor.NewServer(motorSvc2))

	servoSvc, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	servopb.RegisterServoServiceServer(gServer1, servo.NewServer(servoSvc))

	servoSvc2, err := subtype.New(map[resource.Name]interface{}{servo.Named("servo1"): injectServo})
	test.That(t, err, test.ShouldBeNil)
	servopb.RegisterServoServiceServer(gServer2, servo.NewServer(servoSvc2))

	sensorSvc, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	sensorpb.RegisterSensorServiceServer(gServer1, sensor.NewServer(sensorSvc))

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

	cfg := config.Config{
		Components: []config.Component{
			{
				Name: "a",
				Type: arm.SubtypeName,
				Frame: &config.Frame{
					Parent:      "b",
					Translation: r3.Vector{X: 1, Y: 2, Z: 3},
					Orientation: &spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: 7},
				},
			},
			{
				Name: "b",
				Type: base.SubtypeName,
			},
		},
	}
	injectRobot1.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return &cfg, nil
	}

	client, err := New(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	arm1, err := arm.FromRobot(client, "arm1")
	test.That(t, err, test.ShouldBeNil)
	_, err = arm1.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = arm1.JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToPosition(context.Background(), spatialmath.NewPoseFromPoint(r3.Vector{X: 1}), &commonpb.WorldState{}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToJointPositions(context.Background(), &armpb.JointPositions{Values: []float64{1}}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	board1, err := board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldNotBeNil)
	test.That(t, board1.ModelAttributes(), test.ShouldResemble, board.ModelAttributes{Remote: true})

	_, err = board1.Status(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	camera1, err := camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera.ReadImage(context.Background(), camera1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	gripper1, err := gripper.FromRobot(client, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	err = gripper1.Open(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_, err = gripper1.Grab(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	motor1, err := motor.FromRobot(client, "motor1")
	test.That(t, err, test.ShouldBeNil)
	err = motor1.SetPower(context.Background(), 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")
	err = motor1.GoFor(context.Background(), 0, 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")

	sensorDevice, err := sensor.FromRobot(client, "sensor1")
	test.That(t, err, test.ShouldBeNil)
	_, err = sensorDevice.Readings(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no generic sensor")

	servo1, err := servo.FromRobot(client, "servo1")
	test.That(t, err, test.ShouldBeNil)
	err = servo1.Move(context.Background(), 5, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	_, err = servo1.Position(context.Background(), nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	resource1, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = resource1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = resource1.(arm.Arm).JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToPosition(context.Background(), spatialmath.NewPoseFromPoint(r3.Vector{X: 1}), &commonpb.WorldState{}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), &armpb.JointPositions{Values: []float64{1}}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

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
	frame, _, err := camera.ReadImage(context.Background(), camera1)
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
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}

	var callCount int
	calledEnough := make(chan struct{})

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		if callCount == 5 {
			close(calledEnough)
		}
		callCount++

		if callCount >= 5 {
			return finalResources
		}
		return emptyResources
	}

	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	start := time.Now()
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	<-calledEnough
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 5*dur)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 10*dur)

	armNames := []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
	baseNames := []resource.Name{base.Named("base2"), base.Named("base3")}
	boardNames := []resource.Name{board.Named("board2"), board.Named("board3")}
	cameraNames := []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	gripperNames := []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}
	motorNames := []resource.Name{motor.Named("motor2"), motor.Named("motor3")}
	servoNames := []resource.Name{servo.Named("servo2"), servo.Named("servo3")}

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
	test.That(t,
		utils.NewStringSet(board.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(boardNames...)...),
	)
	test.That(t,
		utils.NewStringSet(camera.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(gripper.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)
	test.That(t,
		utils.NewStringSet(motor.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(motorNames...)...),
	)
	test.That(t,
		utils.NewStringSet(sensor.NamesFromRobot(client)...),
		test.ShouldBeEmpty,
	)
	test.That(t,
		utils.NewStringSet(servo.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(servoNames...)...),
	)
	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			motorNames,
			servoNames,
		)...))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return emptyResources }
	client, err = New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm1")}
	baseNames = []resource.Name{base.Named("base1")}
	boardNames = []resource.Name{board.Named("board1"), board.Named("board3")}
	cameraNames = []resource.Name{camera.Named("camera1")}
	gripperNames = []resource.Name{gripper.Named("gripper1")}

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
	test.That(t,
		utils.NewStringSet(board.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(boardNames...)...),
	)
	test.That(t,
		utils.NewStringSet(camera.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(gripper.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)

	test.That(t,
		utils.NewStringSet(sensor.NamesFromRobot(client)...),
		test.ShouldBeEmpty,
	)
	test.That(t,
		utils.NewStringSet(servo.NamesFromRobot(client)...),
		test.ShouldBeEmpty,
	)

	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
		)...))

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }
	test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
	baseNames = []resource.Name{base.Named("base2"), base.Named("base3")}
	boardNames = []resource.Name{board.Named("board2"), board.Named("board3")}
	cameraNames = []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	gripperNames = []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}

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
	test.That(t,
		utils.NewStringSet(board.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(boardNames...)...),
	)
	test.That(t,
		utils.NewStringSet(camera.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(gripper.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)
	test.That(t,
		utils.NewStringSet(motor.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(motorNames...)...),
	)
	test.That(t,
		utils.NewStringSet(sensor.NamesFromRobot(client)...),
		test.ShouldBeEmpty,
	)
	test.That(t,
		utils.NewStringSet(servo.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(servoNames...)...),
	)
	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			motorNames,
			servoNames,
		)...))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDisconnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1")}
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
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
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
	logger := golog.NewTestLogger(t)
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
			if strings.HasSuffix(info.FullMethod, "RobotService/GetStatus") {
				if unaryStatusCallReceived {
					return nil, status.Error(codes.Unknown, io.ErrClosedPipe.Error())
				}
				unaryStatusCallReceived = true
			}
			var resp interface{}
			return resp, nil
		},
	)
	gServer := grpc.NewServer(justOneUnaryStatusCall)

	injectRobot := &inject.Robot{}
	injectRobot.StatusFunc = func(ctx context.Context, rs []resource.Name) ([]robot.Status, error) {
		return []robot.Status{}, nil
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

	t.Run("unary call to connected remote", func(t *testing.T) {
		t.Helper()

		client.connected = false
		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, unaryStatusCallReceived, test.ShouldBeFalse)
		client.connected = true
	})

	t.Run("unary call to disconnected remote", func(t *testing.T) {
		t.Helper()

		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("unary call to undetected disconnected remote", func(t *testing.T) {
		test.That(t, unaryStatusCallReceived, test.ShouldBeTrue)
		_, err = client.Status(context.Background(), []resource.Name{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
	})

	defer func() {
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}()
	gServer.Stop()
}

func TestClientStreamDisconnectHandler(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return nil }
	injectRobot.StatusFunc = func(ctx context.Context, rs []resource.Name) ([]robot.Status, error) {
		return []robot.Status{}, nil
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

		client.connected = false
		_, err = client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		test.That(t, streamStatusCallReceived, test.ShouldBeFalse)
		client.connected = true
	})

	t.Run("stream call to connected remote", func(t *testing.T) {
		t.Helper()

		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		ssc.Recv()
		test.That(t, streamStatusCallReceived, test.ShouldBeTrue)
	})

	t.Run("receive call from stream of disconnected remote", func(t *testing.T) {
		t.Helper()

		ssc, err := client.client.StreamStatus(context.Background(), &pb.StreamStatusRequest{})
		test.That(t, err, test.ShouldBeNil)

		client.connected = false
		_, err = ssc.Recv()
		test.That(t, status.Code(err), test.ShouldEqual, codes.Unavailable)
		test.That(t, err.Error(), test.ShouldContainSubstring, fmt.Sprintf("not connected to remote robot at %s", listener.Addr().String()))
		client.connected = true
	})

	defer func() {
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}()
	gServer.Stop()
}

type mockType struct {
	reconfCount int64
}

func (mt *mockType) Name() resource.Name {
	return resource.Name{}
}

func (mt *mockType) Reconfigure(ctx context.Context, newRes resource.Reconfigurable) error {
	atomic.AddInt64(&mt.reconfCount, 1)
	return nil
}

func TestClientReconnect(t *testing.T) {
	someSubtype := resource.NewSubtype(
		resource.Namespace("acme"),
		resource.ResourceTypeComponent,
		resource.SubtypeName(uuid.New().String()),
	)
	var called int64
	registry.RegisterResourceSubtype(
		someSubtype,
		registry.ResourceSubtype{
			RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
				atomic.AddInt64(&called, 1)
				return &mockType{}
			},
		},
	)

	logger := golog.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	thing1Name := resource.NameFromSubtype(someSubtype, "thing1")
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1"), thing1Name}
	}

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc2, err := subtype.New(map[resource.Name]interface{}{arm.Named("arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	armpb.RegisterArmServiceServer(gServer, arm.NewServer(armSvc2))

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
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}()

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)
	thing1Client, err := client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)
	a, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = a.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
	test.That(t, atomic.LoadInt64(&thing1Client.(*mockType).reconfCount), test.ShouldEqual, 0)

	gServer.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())

	gServer2 := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot))
	armpb.RegisterArmServiceServer(gServer2, arm.NewServer(armSvc2))

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

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 2)
	test.That(t, atomic.LoadInt64(&thing1Client.(*mockType).reconfCount), test.ShouldEqual, 1)
}

func TestClientRefreshNoReconfigure(t *testing.T) {
	someSubtype := resource.NewSubtype(
		resource.Namespace("acme"),
		resource.ResourceTypeComponent,
		resource.SubtypeName(uuid.New().String()),
	)
	var called int64
	registry.RegisterResourceSubtype(
		someSubtype,
		registry.ResourceSubtype{
			RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
				atomic.AddInt64(&called, 1)
				return &mockType{}
			},
		},
	)

	logger := golog.NewTestLogger(t)

	var listener net.Listener = gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	thing1Name := resource.NameFromSubtype(someSubtype, "thing1")

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
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	}()

	thing1Client, err := client.ResourceByName(thing1Name)
	test.That(t, err, test.ShouldBeNil)

	close(allow)
	<-calledEnough

	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 2)

	test.That(t, atomic.LoadInt64(&called), test.ShouldEqual, 1)
	test.That(t, atomic.LoadInt64(&thing1Client.(*mockType).reconfCount), test.ShouldEqual, 0)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
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

	respWith := []resource.RPCSubtype{
		{
			Subtype: resource.NewSubtype("acme", resource.ResourceTypeComponent, "huwat"),
			Desc:    desc1,
		},
		{
			Subtype: resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat"),
			Desc:    desc2,
		},
	}

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return respWith }
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
	resources, rpcSubtypes, err := client.resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, finalResources)
	test.That(t, rpcSubtypes, test.ShouldBeEmpty)

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

	resources, rpcSubtypes, err = client.resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, finalResources)

	test.That(t, rpcSubtypes, test.ShouldHaveLength, len(respWith))
	for idx, rpcType := range rpcSubtypes {
		otherT := respWith[idx]
		test.That(t, rpcType.Subtype, test.ShouldResemble, otherT.Subtype)
		test.That(t, rpcType.Desc.AsProto(), test.ShouldResemble, otherT.Desc.AsProto())
	}

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDiscovery(t *testing.T) {
	injectRobot := &inject.Robot{}
	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return finalResources
	}
	q := discovery.Query{movementsensor.Named("foo").ResourceSubtype, "something"}
	injectRobot.DiscoverComponentsFunc = func(ctx context.Context, keys []discovery.Query) ([]discovery.Discovery, error) {
		return []discovery.Discovery{{
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

	resp, err := client.DiscoverComponents(context.Background(), []discovery.Query{q})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 1)
	test.That(t, resp[0].Query, test.ShouldResemble, q)
	test.That(t, resp[0].Results, test.ShouldResemble, map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}})

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func ensurePartsAreEqual(part, otherPart *config.FrameSystemPart) error {
	if part.Name != otherPart.Name {
		return errors.Errorf("part had name %s while other part had name %s", part.Name, otherPart.Name)
	}
	frameConfig := part.FrameConfig
	otherFrameConfig := otherPart.FrameConfig
	if frameConfig.Parent != otherFrameConfig.Parent {
		return errors.Errorf("part had parent %s while other part had parent %s", frameConfig.Parent, otherFrameConfig.Parent)
	}
	if !spatialmath.R3VectorAlmostEqual(frameConfig.Translation, otherFrameConfig.Translation, 1e-8) {
		return errors.New("translations of parts not equal")
	}
	orient := frameConfig.Orientation
	otherOrient := otherFrameConfig.Orientation

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
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer := grpc.NewServer()
	failingServer := grpc.NewServer()

	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	workingRobot := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	failingRobot := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}

	fsConfigs := []*config.FrameSystemPart{
		{
			Name: "frame1",
			FrameConfig: &config.Frame{
				Parent:      referenceframe.World,
				Translation: r3.Vector{X: 1, Y: 2, Z: 3},
				Orientation: &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1},
			},
		},
		{
			Name: "frame2",
			FrameConfig: &config.Frame{
				Parent:      "frame1",
				Translation: r3.Vector{X: 1, Y: 2, Z: 3},
			},
		},
	}

	workingRobot.FrameSystemConfigFunc = func(
		ctx context.Context,
		additionalTransforms []*commonpb.Transform,
	) (framesystemparts.Parts, error) {
		return framesystemparts.Parts(fsConfigs), nil
	}
	configErr := errors.New("failed to retrieve config")
	failingRobot.FrameSystemConfigFunc = func(
		ctx context.Context,
		additionalTransforms []*commonpb.Transform,
	) (framesystemparts.Parts, error) {
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
		frameSystemParts, err := workingFSClient.FrameSystemConfig(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], frameSystemParts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], frameSystemParts[1])
		test.That(t, err, test.ShouldBeNil)
	})

	err = workingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for working frame service", func(t *testing.T) {
		workingDialedClient, err := New(ctx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		frameSystemParts, err := workingDialedClient.FrameSystemConfig(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[0], frameSystemParts[0])
		test.That(t, err, test.ShouldBeNil)
		err = ensurePartsAreEqual(fsConfigs[1], frameSystemParts[1])
		test.That(t, err, test.ShouldBeNil)
		err = workingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	failingFSClient, err := New(ctx, listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client test config for failing frame service", func(t *testing.T) {
		frameSystemParts, err := failingFSClient.FrameSystemConfig(ctx, nil)
		test.That(t, frameSystemParts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	err = failingFSClient.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	t.Run("dialed client test config for failing frame service with failing config", func(t *testing.T) {
		failingDialedClient, err := New(ctx, listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		parts, err := failingDialedClient.FrameSystemConfig(ctx, nil)
		test.That(t, parts, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingDialedClient.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestClientStatus(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	gServer2 := grpc.NewServer()

	injectRobot := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return []resource.Name{} },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	injectRobot2 := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return []resource.Name{} },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	go gServer.Serve(listener1)
	defer gServer.Stop()

	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = New(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working status service", func(t *testing.T) {
		client, err := New(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		gStatus := robot.Status{Name: movementsensor.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := robot.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statusMap := map[resource.Name]robot.Status{
			gStatus.Name: gStatus,
			aStatus.Name: aStatus,
		}
		injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
			statuses := make([]robot.Status, 0, len(resourceNames))
			for _, n := range resourceNames {
				statuses = append(statuses, statusMap[n])
			}
			return statuses, nil
		}
		expected := map[resource.Name]interface{}{
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		resp, err := client.Status(context.Background(), []resource.Name{aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])

		result := struct{}{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &result})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(resp[0].Status)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, aStatus.Status)

		resp, err = client.Status(context.Background(), []resource.Name{gStatus.Name, aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)

		observed := map[resource.Name]interface{}{
			resp[0].Name: resp[0].Status,
			resp[1].Name: resp[1].Status,
		}
		test.That(t, observed, test.ShouldResemble, expected)

		err = client.Close(context.Background())
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing status client", func(t *testing.T) {
		client2, err := New(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		passedErr := errors.New("can't get status")
		injectRobot2.StatusFunc = func(ctx context.Context, status []resource.Name) ([]robot.Status, error) {
			return nil, passedErr
		}
		_, err = client2.Status(context.Background(), []resource.Name{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestForeignResource(t *testing.T) {
	injectRobot := &inject.Robot{}

	desc1, err := grpcreflect.LoadServiceDescriptor(&pb.RobotService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	desc2, err := grpcreflect.LoadServiceDescriptor(&armpb.ArmService_ServiceDesc)
	test.That(t, err, test.ShouldBeNil)

	subtype1 := resource.NewSubtype("acme", resource.ResourceTypeComponent, "huwat")
	subtype2 := resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat")
	respWith := []resource.RPCSubtype{
		{
			Subtype: resource.NewSubtype("acme", resource.ResourceTypeComponent, "huwat"),
			Desc:    desc1,
		},
		{
			Subtype: resource.NewSubtype("acme", resource.ResourceTypeComponent, "wat"),
			Desc:    desc2,
		},
	}

	respWithResources := []resource.Name{
		arm.Named("arm1"),
		resource.NameFromSubtype(subtype1, "thing1"),
		resource.NameFromSubtype(subtype2, "thing2"),
	}

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return respWith }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return respWithResources }

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	reflection.Register(gServer)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

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
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	var callCount int

	injectRobot.ResourceRPCSubtypesFunc = func() []resource.RPCSubtype { return nil }
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

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientStopAll(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	stopAllCalled := false
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:       resourcesFunc,
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
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
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return validResources },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := subtype.New(map[resource.Name]interface{}{arm.Named("remote:arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	armpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc1))

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
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	validResources := []resource.Name{arm.Named("remote1:arm1"), arm.Named("remote2:arm1")}
	injectRobot1 := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return validResources },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
	}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose1, nil
	}

	armSvc1, err := subtype.New(map[resource.Name]interface{}{arm.Named("remote1:arm1"): injectArm, arm.Named("remote2:arm1"): injectArm})
	test.That(t, err, test.ShouldBeNil)
	armpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc1))

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
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(arm.Named("arm1")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientOperationIntercept(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return []resource.Name{} },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
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

	injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
		meta, ok := metadata.FromIncomingContext(ctx)
		test.That(t, ok, test.ShouldBeTrue)
		receivedOpID, err := operation.GetOrCreateFromMetadata(meta)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedOpID.String(), test.ShouldEqual, fakeOp.ID.String())
		return []robot.Status{}, nil
	}

	resp, err := client.Status(ctx, []resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 0)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestGetUnknownResource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{
		ResourceNamesFunc:       func() []resource.Name { return []resource.Name{arm.Named("myArm")} },
		ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
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
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(base.Named("notABase")))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
