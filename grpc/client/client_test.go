package client

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	armpb "go.viam.com/rdk/proto/api/component/arm/v1"
	basepb "go.viam.com/rdk/proto/api/component/base/v1"
	boardpb "go.viam.com/rdk/proto/api/component/board/v1"
	camerapb "go.viam.com/rdk/proto/api/component/camera/v1"
	gripperpb "go.viam.com/rdk/proto/api/component/gripper/v1"
	inputcontrollerpb "go.viam.com/rdk/proto/api/component/inputcontroller/v1"
	motorpb "go.viam.com/rdk/proto/api/component/motor/v1"
	sensorpb "go.viam.com/rdk/proto/api/component/sensor/v1"
	servopb "go.viam.com/rdk/proto/api/component/servo/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	framepb "go.viam.com/rdk/proto/api/service/framesystem/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
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

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	resourcesFunc := func() []resource.Name { return []resource.Name{} }
	injectRobot1 := &inject.Robot{ResourceNamesFunc: resourcesFunc}
	injectRobot2 := &inject.Robot{ResourceNamesFunc: resourcesFunc}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	pose1 := &commonpb.Pose{
		X:     0.0,
		Y:     0.0,
		Z:     0.0,
		Theta: 0.0,
		OX:    1.0,
		OY:    0.0,
		OZ:    0.0,
	}
	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pose1, nil
	}

	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
		return nil, errors.New("no status")
	}

	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, jpeg.Encode(&imgBuf, img, nil), test.ShouldBeNil)

	var imageReleased bool
	injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return img, func() { imageReleased = true }, nil
	}

	injectInputDev := &inject.InputController{}
	injectInputDev.GetControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}

	injectGripper := &inject.Gripper{}
	var gripperOpenCalled bool
	injectGripper.OpenFunc = func(ctx context.Context) error {
		gripperOpenCalled = true
		return nil
	}
	var gripperGrabCalled bool
	injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
		gripperGrabCalled = true
		return true, nil
	}

	injectServo := &inject.Servo{}
	var capServoAngle uint8
	injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		capServoAngle = angle
		return nil
	}
	injectServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 5, nil
	}

	// for these, just need to double check type (main tests should be in the respective grpc client and server files)
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

	frameSysSvc, err := subtype.New(map[resource.Name]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	framepb.RegisterFrameSystemServiceServer(gServer1, framesystem.NewServer(frameSysSvc))

	frameSysSvc2, err := subtype.New(map[resource.Name]interface{}{framesystem.Name: "not a frame system"})
	test.That(t, err, test.ShouldBeNil)
	framepb.RegisterFrameSystemServiceServer(gServer2, framesystem.NewServer(frameSysSvc2))

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
					Translation: spatialmath.TranslationConfig{X: 1, Y: 2, Z: 3},
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
	_, err = arm1.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = arm1.GetJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToPosition(context.Background(), &commonpb.Pose{X: 1}, &commonpb.WorldState{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToJointPositions(context.Background(), &armpb.JointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	board1, err := board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldNotBeNil)
	test.That(t, board1.ModelAttributes(), test.ShouldResemble, board.ModelAttributes{Remote: true})

	_, err = board1.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	camera1, err := camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	_, _, err = camera1.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	gripper1, err := gripper.FromRobot(client, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	err = gripper1.Open(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_, err = gripper1.Grab(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	motor1, err := motor.FromRobot(client, "motor1")
	test.That(t, err, test.ShouldBeNil)
	err = motor1.SetPower(context.Background(), 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")
	err = motor1.GoFor(context.Background(), 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")

	sensorDevice, err := sensor.FromRobot(client, "sensor1")
	test.That(t, err, test.ShouldBeNil)
	_, err = sensorDevice.GetReadings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no generic sensor")

	servo1, err := servo.FromRobot(client, "servo1")
	test.That(t, err, test.ShouldBeNil)
	err = servo1.Move(context.Background(), 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	_, err = servo1.GetPosition(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	resource1, err := client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = resource1.(arm.Arm).GetEndPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = resource1.(arm.Arm).GetJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToPosition(context.Background(), &commonpb.Pose{X: 1}, &commonpb.WorldState{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), &armpb.JointPositions{Degrees: []float64{1}})
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
	pos, err := arm1.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, pose1.String())

	_, err = base.FromRobot(client, "base1")
	test.That(t, err, test.ShouldBeNil)

	_, err = base.FromRobot(client, "base2")
	test.That(t, err, test.ShouldBeNil)

	_, err = base.FromRobot(client, "base3")
	test.That(t, err, test.ShouldBeNil)

	_, err = board.FromRobot(client, "board1")
	test.That(t, err, test.ShouldBeNil)

	_, err = board.FromRobot(client, "board3")
	test.That(t, err, test.ShouldBeNil)

	camera1, err = camera.FromRobot(client, "camera1")
	test.That(t, err, test.ShouldBeNil)
	frame, _, err := camera1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
	test.That(t, imageReleased, test.ShouldBeTrue)

	gripper1, err = gripper.FromRobot(client, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	err = gripper1.Open(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)

	inputDev, err := input.FromRobot(client, "inputController1")
	test.That(t, err, test.ShouldBeNil)
	controlList, err := inputDev.GetControls(context.Background())
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
	err = servo1.Move(context.Background(), 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)

	currentVal, err := servo1.GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currentVal, test.ShouldEqual, 5)

	resource1, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	pos, err = resource1.(arm.Arm).GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, pose1.String())

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
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, dur)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 2*dur)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())
}

func TestClientReconnect(t *testing.T) {
	logger := golog.NewTestLogger(t)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr := fmt.Sprintf("localhost:%d", port)

	listener, err := net.Listen("tcp", addr)
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("arm1")}
	}

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
	gServer.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 0)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError, client.checkConnected())

	gServer2 := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot))

	listener, err = net.Listen("tcp", addr)
	test.That(t, err, test.ShouldBeNil)
	go gServer2.Serve(listener)
	defer gServer2.Stop()

	test.That(t, <-client.Changed(), test.ShouldBeTrue)
	test.That(t, client.Connected(), test.ShouldBeTrue)
	test.That(t, len(client.ResourceNames()), test.ShouldEqual, 1)
	_, err = client.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
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
	injectRobot.ResourceNamesFunc = func() []resource.Name { return finalResources }

	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	resources, err := client.resources(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resources, test.ShouldResemble, finalResources)

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDiscovery(t *testing.T) {
	injectRobot := &inject.Robot{}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return finalResources
	}
	key := discovery.Key{imu.Named("imu").ResourceSubtype, "some imu"}
	injectRobot.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
		return []discovery.Discovery{{
			Key:        key,
			Discovered: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}},
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

	resp, err := client.Discover(context.Background(), []discovery.Key{key})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp), test.ShouldEqual, 1)
	test.That(t, resp[0].Key, test.ShouldResemble, key)
	test.That(t, resp[0].Discovered, test.ShouldResemble, map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}})

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
