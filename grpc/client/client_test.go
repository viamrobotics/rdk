package client

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"math"
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
	_ "go.viam.com/rdk/component/base/register"
	"go.viam.com/rdk/component/board"
	_ "go.viam.com/rdk/component/board/register"
	"go.viam.com/rdk/component/camera"
	_ "go.viam.com/rdk/component/camera/register"
	"go.viam.com/rdk/component/gripper"
	_ "go.viam.com/rdk/component/gripper/register"
	"go.viam.com/rdk/component/input"
	_ "go.viam.com/rdk/component/input/register"
	"go.viam.com/rdk/component/motor"
	_ "go.viam.com/rdk/component/motor/register"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	metadataserver "go.viam.com/rdk/grpc/metadata/server"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
	metadatapb "go.viam.com/rdk/proto/api/service/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var emptyStatus = &pb.Status{
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
	Sensors: map[string]*pb.SensorStatus{},
	Motors: map[string]*pb.MotorStatus{
		"motor1": {},
		"motor2": {},
	},
	InputControllers: map[string]*pb.InputControllerStatus{
		"inputController1": {},
	},
	Servos: map[string]*pb.ServoStatus{
		"servo1": {},
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
		"board3": {},
	},
}

var emptyResources = []resource.Name{
	arm.Named("arm1"),
	base.Named("base1"),
	board.Named("board1"),
	board.Named("board3"),
	gripper.Named("gripper1"),
	camera.Named("camera1"),
}

var finalStatus = &pb.Status{
	Arms: map[string]*pb.ArmStatus{
		"arm2": {
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
		"arm3": {
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
		"base2": true,
		"base3": true,
	},
	Grippers: map[string]bool{
		"gripper2": true,
		"gripper3": true,
	},
	Cameras: map[string]bool{
		"camera2": true,
		"camera3": true,
	},
	Sensors: map[string]*pb.SensorStatus{},
	Servos: map[string]*pb.ServoStatus{
		"servo2": {},
		"servo3": {},
	},
	Motors: map[string]*pb.MotorStatus{
		"motor2": {},
		"motor3": {},
	},
	InputControllers: map[string]*pb.InputControllerStatus{
		"inputController2": {},
		"inputController3": {},
	},
	Boards: map[string]*commonpb.BoardStatus{
		"board2": {
			Analogs: map[string]*commonpb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
		"board3": {
			Analogs: map[string]*commonpb.AnalogStatus{
				"analog1": {},
				"analog2": {},
			},
			DigitalInterrupts: map[string]*commonpb.DigitalInterruptStatus{
				"encoder":  {},
				"digital1": {},
			},
		},
	},
}

var finalResources = []resource.Name{
	arm.Named("arm2"),
	arm.Named("arm3"),
	base.Named("base2"),
	base.Named("base3"),
	board.Named("board2"),
	board.Named("board3"),
	servo.Named("servo2"),
	servo.Named("servo3"),
	gripper.Named("gripper2"),
	gripper.Named("gripper3"),
	camera.Named("camera2"),
	camera.Named("camera3"),
	motor.Named("motor2"),
	motor.Named("motor3"),
}

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()
	gServer2 := grpc.NewServer()
	injectRobot1 := &inject.Robot{}
	injectRobot2 := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer1, server.New(injectRobot1))
	pb.RegisterRobotServiceServer(gServer2, server.New(injectRobot2))

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return nil, errors.New("whoops")
	}
	injectRobot1.BaseByNameFunc = func(name string) (base.Base, bool) {
		return nil, false
	}
	injectRobot1.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
		return nil, false
	}
	injectRobot1.BoardByNameFunc = func(name string) (board.Board, bool) {
		return nil, false
	}
	injectRobot1.CameraByNameFunc = func(name string) (camera.Camera, bool) {
		return nil, false
	}
	injectRobot1.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return nil, false
	}
	injectRobot1.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return nil, false
	}
	injectRobot1.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		return nil, false
	}
	injectRobot2.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}
	injectBase := &inject.Base{}

	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		pos := emptyStatus.Arms["arm1"].GridPosition
		convertedPos := &commonpb.Pose{
			X: pos.X, Y: pos.Y, Z: pos.Z, OX: pos.OX, OY: pos.OY, OZ: pos.OZ, Theta: pos.Theta,
		}
		return convertedPos, nil
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
	injectBoard := &inject.Board{}
	injectBoard.StatusFunc = func(ctx context.Context) (*commonpb.BoardStatus, error) {
		return nil, errors.New("no status")
	}
	injectMotor := &inject.Motor{}
	injectServo := &inject.Servo{}
	var capServoAngle uint8
	injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		capServoAngle = angle
		return nil
	}
	injectServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 5, nil
	}
	injectRobot2.BoardByNameFunc = func(name string) (board.Board, bool) {
		return injectBoard, true
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

	// for these, just need to double check type (main tests should be in the respective grpc client and server files)
	armSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc1))

	armSvc2, err := subtype.New((map[resource.Name]interface{}{arm.Named("arm1"): injectArm}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer2, arm.NewServer(armSvc2))

	baseSvc, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterBaseServiceServer(gServer1, base.NewServer(baseSvc))

	baseSvc2, err := subtype.New((map[resource.Name]interface{}{base.Named("base1"): injectBase}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterBaseServiceServer(gServer2, base.NewServer(baseSvc2))

	boardSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterBoardServiceServer(gServer1, board.NewServer(boardSvc1))

	boardSvc2, err := subtype.New((map[resource.Name]interface{}{board.Named("board1"): injectBoard}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterBoardServiceServer(gServer2, board.NewServer(boardSvc2))

	gripperSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGripperServiceServer(gServer1, gripper.NewServer(gripperSvc1))

	gripperSvc2, err := subtype.New((map[resource.Name]interface{}{gripper.Named("gripper1"): injectGripper}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGripperServiceServer(gServer2, gripper.NewServer(gripperSvc2))

	servoSvc, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterServoServiceServer(gServer1, servo.NewServer(servoSvc))

	servoSvc2, err := subtype.New((map[resource.Name]interface{}{servo.Named("servo1"): injectServo}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterServoServiceServer(gServer2, servo.NewServer(servoSvc2))

	cameraSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterCameraServiceServer(gServer1, camera.NewServer(cameraSvc1))

	cameraSvc2, err := subtype.New((map[resource.Name]interface{}{camera.Named("camera1"): injectCamera}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterCameraServiceServer(gServer2, camera.NewServer(cameraSvc2))

	inputControllerSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterInputControllerServiceServer(gServer1, input.NewServer(inputControllerSvc1))

	inputControllerSvc2, err := subtype.New((map[resource.Name]interface{}{input.Named("inputController1"): injectInputDev}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterInputControllerServiceServer(gServer2, input.NewServer(inputControllerSvc2))

	motorSvc, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterMotorServiceServer(gServer1, motor.NewServer(motorSvc))

	motorSvc2, err := subtype.New(map[resource.Name]interface{}{motor.Named("motor1"): injectMotor, motor.Named("motor2"): injectMotor})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterMotorServiceServer(gServer2, motor.NewServer(motorSvc2))

	sensorSvc, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterSensorServiceServer(gServer1, sensor.NewServer(sensorSvc))

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

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Boards: map[string]*commonpb.BoardStatus{
				"board1": {},
				"board2": {},
			},
		}, nil
	}

	cfg := config.Config{
		Components: []config.Component{
			{
				Name: "a",
				Type: config.ComponentTypeArm,
				Frame: &config.Frame{
					Parent:      "b",
					Translation: spatialmath.Translation{X: 1, Y: 2, Z: 3},
					Orientation: &spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1.0000000000000002, Theta: 7},
				},
			},
			{
				Name: "b",
				Type: config.ComponentTypeBase,
			},
		},
	}
	injectRobot1.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return &cfg, nil
	}

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
				Translation: spatialmath.Translation{4, 5, 6},
			},
		},
	}
	fss := &inject.FrameSystemService{}
	fss.FrameSystemConfigFunc = func(ctx context.Context) ([]*config.FrameSystemPart, error) {
		return fsConfigs, nil
	}
	injectRobot1.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		services := make(map[resource.Name]interface{})
		services[framesystem.Name] = fss
		service, ok := services[name]
		return service, ok
	}

	client, err := New(context.Background(), listener1.Addr().String(), logger, WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)

	newCfg, err := client.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newCfg.Components[0], test.ShouldResemble, cfg.Components[0])
	test.That(t, newCfg.Components[1], test.ShouldResemble, cfg.Components[1])
	test.That(t, newCfg.Components[1].Frame, test.ShouldBeNil)

	// test robot frame system
	frameSys, err := client.FrameSystem(context.Background(), "", "")
	test.That(t, err, test.ShouldBeNil)
	frame1 := frameSys.GetFrame("frame1")
	frame1Offset := frameSys.GetFrame("frame1_offset")
	frame2 := frameSys.GetFrame("frame2")
	frame2Offset := frameSys.GetFrame("frame2_offset")

	resFrame, err := frameSys.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame2Offset)
	resFrame, err = frameSys.Parent(frame2Offset)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1)
	resFrame, err = frameSys.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1Offset)
	resFrame, err = frameSys.Parent(frame1Offset)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frameSys.World())

	// test status
	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return nil, errors.New("whoops")
	}
	_, err = client.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	_, ok := client.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)

	arm1, ok := arm.FromRobot(client, "arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err = arm1.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = arm1.GetJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToPosition(context.Background(), &commonpb.Pose{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToJointPositions(context.Background(), &componentpb.ArmJointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	gripper1, ok := client.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	err = gripper1.Open(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_, err = gripper1.Grab(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	servo1, ok := servo.FromRobot(client, "servo1")
	test.That(t, ok, test.ShouldBeTrue)
	err = servo1.Move(context.Background(), 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	_, err = servo1.GetPosition(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	motor1, ok := client.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	err = motor1.Go(context.Background(), 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")
	err = motor1.GoFor(context.Background(), 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no motor")

	board1, ok := client.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1, test.ShouldNotBeNil)

	test.That(t, board1.ModelAttributes(), test.ShouldResemble, board.ModelAttributes{Remote: true})

	_, err = board1.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	camera1, ok := client.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, _, err = camera1.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	sensorDevice, ok := sensor.FromRobot(client, "sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err = sensorDevice.GetReadings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no generic sensor")

	resource1, ok := client.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, err = resource1.(arm.Arm).GetEndPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = resource1.(arm.Arm).GetJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToPosition(context.Background(), &commonpb.Pose{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), &componentpb.ArmJointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// working
	client, err = New(context.Background(), listener2.Addr().String(), logger, WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, emptyStatus.String())

	_, ok = client.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = client.BaseByName("base2")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = client.BaseByName("base3")
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, func() { client.RemoteByName("remote1") }, test.ShouldPanic)

	arm1, ok = arm.FromRobot(client, "arm1")
	test.That(t, ok, test.ShouldBeTrue)
	pos, err := arm1.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	gripper1, ok = client.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	err = gripper1.Open(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)
	gripperOpenCalled = false

	servo1, ok = servo.FromRobot(client, "servo1")
	test.That(t, ok, test.ShouldBeTrue)
	err = servo1.Move(context.Background(), 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)

	currentVal, err := servo1.GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currentVal, test.ShouldEqual, 5)

	motor1, ok = client.MotorByName("motor1")
	test.That(t, motor1, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	motor2, ok := client.MotorByName("motor2")
	test.That(t, motor2, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	board1, ok = client.BoardByName("board1")
	test.That(t, board1, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = client.BoardByName("board3")
	test.That(t, ok, test.ShouldBeTrue)

	camera1, ok = client.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	frame, _, err := camera1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
	test.That(t, imageReleased, test.ShouldBeTrue)

	inputDev, ok := client.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	controlList, err := inputDev.GetControls(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})

	resource1, ok = client.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	pos, err = resource1.(arm.Arm).GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientRefresh(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectMetadata := &inject.Metadata{}
	metadatapb.RegisterMetadataServiceServer(gServer, metadataserver.New(injectMetadata))

	go gServer.Serve(listener)
	defer gServer.Stop()

	var callCount int
	calledEnough := make(chan struct{})
	var shouldError bool
	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		if shouldError {
			return nil, errors.New("no more for you")
		}
		if callCount > 5 {
			shouldError = true
			close(calledEnough)
		}
		callCount++
		if callCount > 5 {
			return finalStatus, nil
		}
		return emptyStatus, nil
	}

	injectMetadata.AllFunc = func() []resource.Name {
		if callCount > 5 {
			return finalResources
		}
		return emptyResources
	}

	start := time.Now()
	dur := 100 * time.Millisecond
	client, err := New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
		WithDialOptions(rpc.WithInsecure()),
	)
	test.That(t, err, test.ShouldBeNil)
	<-calledEnough
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 5*dur)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 10*dur)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, finalStatus.String())

	armNames := []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
	boardNames := []resource.Name{board.Named("board2"), board.Named("board3")}
	baseNames := []resource.Name{base.Named("base2"), base.Named("base3")}

	gripperNames := []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}
	cameraNames := []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	servoNames := []resource.Name{servo.Named("servo2"), servo.Named("servo3")}
	motorNames := []resource.Name{motor.Named("motor2"), motor.Named("motor3")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(arm.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(armNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(boardNames...)...),
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
	test.That(t,
		utils.NewStringSet(client.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(motorNames...)...),
	)
	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			baseNames,
		)...))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}

	injectMetadata.AllFunc = func() []resource.Name {
		return emptyResources
	}
	client, err = New(
		context.Background(),
		listener.Addr().String(),
		logger,
		WithRefreshEvery(dur),
		WithDialOptions(rpc.WithInsecure()),
	)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm1")}
	boardNames = []resource.Name{board.Named("board1"), board.Named("board3")}
	baseNames = []resource.Name{base.Named("base1")}
	gripperNames = []resource.Name{gripper.Named("gripper1")}
	cameraNames = []resource.Name{camera.Named("camera1")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(arm.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(armNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(boardNames...)...),
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
			boardNames,
			gripperNames,
			cameraNames,
			baseNames,
		)...))

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return finalStatus, nil
	}
	injectMetadata.AllFunc = func() []resource.Name {
		return finalResources
	}
	test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
	baseNames = []resource.Name{base.Named("base2"), base.Named("base3")}
	boardNames = []resource.Name{board.Named("board2"), board.Named("board3")}
	gripperNames = []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}
	cameraNames = []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(arm.NamesFromRobot(client)...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(armNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(gripperNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(cameraNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(baseNames...)...),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board2", "board3"),
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
	test.That(t,
		utils.NewStringSet(client.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(testutils.ExtractNames(motorNames...)...),
	)
	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			baseNames,
		)...))

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	injectMetadata := &inject.Metadata{}
	metadatapb.RegisterMetadataServiceServer(gServer, metadataserver.New(injectMetadata))

	go gServer.Serve(listener)
	defer gServer.Stop()

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}

	injectMetadata.AllFunc = func() []resource.Name {
		return emptyResources
	}

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := New(ctx, listener.Addr().String(), logger, WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	client2, err := New(ctx, listener.Addr().String(), logger, WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 4)

	err = client1.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
