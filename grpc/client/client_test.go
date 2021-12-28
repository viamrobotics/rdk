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

	"go.viam.com/rdk/base"
	"go.viam.com/rdk/component/arm"
	_ "go.viam.com/rdk/component/arm/register"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/camera"
	_ "go.viam.com/rdk/component/camera/register"
	"go.viam.com/rdk/component/forcematrix"
	"go.viam.com/rdk/component/gripper"
	_ "go.viam.com/rdk/component/gripper/register"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/motor"
	_ "go.viam.com/rdk/component/motor/register"
	"go.viam.com/rdk/component/servo"
	_ "go.viam.com/rdk/component/servo/register"
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
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/compass"
	servicepkg "go.viam.com/rdk/services"
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
	Sensors: map[string]*pb.SensorStatus{
		"compass1": {
			Type: compass.Type,
		},
		"compass2": {
			Type: compass.RelativeType,
		},
		"fsm1": {
			Type: string(forcematrix.SubtypeName),
		},
		"fsm2": {
			Type: string(forcematrix.SubtypeName),
		},
	},
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
	Boards: map[string]*pb.BoardStatus{
		"board1": {
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
		"board3": {},
	},
}

var emptyResources = []resource.Name{arm.Named("arm1"), gripper.Named("gripper1"), camera.Named("camera1")}

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
	Sensors: map[string]*pb.SensorStatus{
		"compass2": {
			Type: compass.Type,
		},
		"compass3": {
			Type: compass.Type,
		},
		"compass4": {
			Type: compass.RelativeType,
		},
		"fsm1": {
			Type: string(forcematrix.SubtypeName),
		},
		"fsm2": {
			Type: string(forcematrix.SubtypeName),
		},
	},
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
	Boards: map[string]*pb.BoardStatus{
		"board2": {
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
		"board3": {
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
				"analog2": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder":  {},
				"digital1": {},
			},
		},
	},
}

var finalResources = []resource.Name{
	arm.Named("arm2"),
	arm.Named("arm3"),
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
	injectRobot1.ArmByNameFunc = func(name string) (arm.Arm, bool) {
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
	injectRobot1.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return nil, false
	}
	injectRobot1.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return nil, false
	}
	injectRobot1.ServoByNameFunc = func(name string) (servo.Servo, bool) {
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
	var (
		capBaseName             string
		capBoardName            string
		capInputControllerName  string
		capAnalogReaderName     string
		capDigitalInterruptName string
		capSensorName           string
	)
	injectBase := &inject.Base{}
	injectBase.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 15, nil
	}
	var baseStopCalled bool
	injectBase.StopFunc = func(ctx context.Context) error {
		baseStopCalled = true
		return nil
	}
	var capBaseMoveArgs []interface{}
	injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
		capBaseMoveArgs = []interface{}{distanceMillis, millisPerSec, block}
		return nil
	}
	var capBaseSpinArgs []interface{}
	injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
		capBaseSpinArgs = []interface{}{angleDeg, degsPerSec, block}
		return nil
	}
	injectRobot2.BaseByNameFunc = func(name string) (base.Base, bool) {
		capBaseName = name
		return injectBase, true
	}
	injectArm := &inject.Arm{}
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
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

	injectAnalogReader := &inject.AnalogReader{}
	injectAnalogReader.ReadFunc = func(ctx context.Context) (int, error) {
		return 6, nil
	}

	injectDigitalInterrupt := &inject.DigitalInterrupt{}
	digitalIntConfig := board.DigitalInterruptConfig{
		Name:    "foo",
		Pin:     "bar",
		Type:    "baz",
		Formula: "baf",
	}
	injectDigitalInterrupt.ConfigFunc = func(ctx context.Context) (board.DigitalInterruptConfig, error) {
		return digitalIntConfig, nil
	}
	injectDigitalInterrupt.ValueFunc = func(ctx context.Context) (int64, error) {
		return 287, nil
	}
	var capDigitalInterruptHigh bool
	var capDigitalInterruptNanos uint64
	injectDigitalInterrupt.TickFunc = func(ctx context.Context, high bool, nanos uint64) error {
		capDigitalInterruptHigh = high
		capDigitalInterruptNanos = nanos
		return nil
	}

	injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
		return emptyStatus.Boards["board1"], nil
	}
	var (
		capGPIOSetPin      string
		capGPIOSetHigh     bool
		capGPIOGetPin      string
		capPWMSetPin       string
		capPWMSetDutyCycle byte
		capPWMSetFreqPin   string
		capPWMSetFreqFreq  uint
	)
	injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
		capGPIOSetPin = pin
		capGPIOSetHigh = high
		return nil
	}
	injectBoard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		capGPIOGetPin = pin
		return true, nil
	}
	injectBoard.PWMSetFunc = func(ctx context.Context, pin string, dutyCycle byte) error {
		capPWMSetPin = pin
		capPWMSetDutyCycle = dutyCycle
		return nil
	}
	injectBoard.PWMSetFreqFunc = func(ctx context.Context, pin string, freq uint) error {
		capPWMSetFreqPin = pin
		capPWMSetFreqFreq = freq
		return nil
	}
	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		capAnalogReaderName = name
		return injectAnalogReader, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		capDigitalInterruptName = name
		return injectDigitalInterrupt, true
	}
	injectRobot2.BoardByNameFunc = func(name string) (board.Board, bool) {
		capBoardName = name
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

	injectFsm := &inject.ForceMatrix{}
	expectedMatrix := make([][]int, 4)
	for i := 0; i < len(expectedMatrix); i++ {
		expectedMatrix[i] = []int{1, 2, 3, 4}
	}
	injectFsm.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		return expectedMatrix, nil
	}
	injectFsm.IsSlippingFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	injectFsm2 := &inject.ForceMatrix{}
	injectFsm2.MatrixFunc = func(ctx context.Context) ([][]int, error) {
		return nil, errors.New("bad matrix")
	}
	injectFsm2.IsSlippingFunc = func(ctx context.Context) (bool, error) {
		return false, errors.New("slip detection error")
	}

	injectCompassDev := &inject.Compass{}
	injectRelCompassDev := &inject.RelativeCompass{}
	injectRobot2.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		capSensorName = name
		switch name {
		case "compass2":
			return injectRelCompassDev, true
		default:
			return injectCompassDev, true
		}
	}

	injectRobot2.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		if name.Subtype == forcematrix.Subtype {
			switch name.Name {
			case "fsm1":
				return injectFsm, true
			case "fsm2":
				return injectFsm2, true
			}
		}
		return nil, false
	}

	injectCompassDev.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{1.2, 2.3}, nil
	}
	injectCompassDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 4.5, nil
	}
	injectCompassDev.StartCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	injectCompassDev.StopCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	injectRelCompassDev.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{1.2, 2.3}, nil
	}
	injectRelCompassDev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 4.5, nil
	}
	injectRelCompassDev.MarkFunc = func(ctx context.Context) error {
		return nil
	}
	injectRelCompassDev.StartCalibrationFunc = func(ctx context.Context) error {
		return nil
	}
	injectRelCompassDev.StopCalibrationFunc = func(ctx context.Context) error {
		return nil
	}

	injectInputDev := &inject.InputController{}
	injectInputDev.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	injectInputDev.LastEventsFunc = func(ctx context.Context) (map[input.Control]input.Event, error) {
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
		eventsOut[input.ButtonStart] = input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonStart, Value: 1.0}
		return eventsOut, nil
	}
	evStream := make(chan input.Event)
	injectInputDev.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		if ctrlFunc != nil {
			outEvent := input.Event{Time: time.Now(), Event: triggers[0], Control: input.ButtonStart, Value: 0.0}
			if control == input.AbsoluteX {
				outEvent = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.75}
			}
			ctrlFunc(ctx, outEvent)
		} else {
			evStream <- input.Event{}
		}
		return nil
	}

	injectRobot2.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		capInputControllerName = name
		return injectInputDev, true
	}

	// for these, just need to double check type (main tests should be in the respective grpc client and server files)
	armSvc1, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer1, arm.NewServer(armSvc1))

	armSvc2, err := subtype.New((map[resource.Name]interface{}{arm.Named("arm1"): injectArm}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterArmServiceServer(gServer2, arm.NewServer(armSvc2))

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

	motorSvc, err := subtype.New((map[resource.Name]interface{}{}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterMotorServiceServer(gServer1, motor.NewServer(motorSvc))

	motorSvc2, err := subtype.New(map[resource.Name]interface{}{motor.Named("motor1"): injectMotor, motor.Named("motor2"): injectMotor})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterMotorServiceServer(gServer2, motor.NewServer(motorSvc2))

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
			Boards: map[string]*pb.BoardStatus{
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
	injectRobot1.ServiceByNameFunc = func(name string) (interface{}, bool) {
		services := make(map[string]interface{})
		services[servicepkg.FrameSystemName] = fss
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

	base1, ok := client.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	err = base1.Stop(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = base1.MoveStraight(context.Background(), 5, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = base1.Spin(context.Background(), 5.2, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	_, err = base1.WidthMillis(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	arm1, ok := client.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err = arm1.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = arm1.CurrentJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToPosition(context.Background(), &commonpb.Pose{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.MoveToJointPositions(context.Background(), &componentpb.ArmJointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = arm1.JointMoveDelta(context.Background(), 0, 0)
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

	servo1, ok := client.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	err = servo1.Move(context.Background(), 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no servo")

	_, err = servo1.AngularOffset(context.Background())
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

	_, ok = client.BoardByName("boardwhat")
	test.That(t, ok, test.ShouldBeFalse)

	_, err = board1.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.GPIOSet(context.Background(), "one", true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	_, err = board1.GPIOGet(context.Background(), "one")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.PWMSet(context.Background(), "one", 1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.PWMSetFreq(context.Background(), "one", 1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	analog1, _ := board1.AnalogReaderByName("analog1")
	_, err = analog1.Read(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	digital1, _ := board1.DigitalInterruptByName("digital1")
	_, err = digital1.Config(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	_, err = digital1.Value(context.Background())
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	err = digital1.Tick(context.Background(), true, 0)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	test.That(t, func() {
		digital1.AddCallback(nil)
	}, test.ShouldPanic)
	test.That(t, func() {
		digital1.AddPostProcessor(nil)
	}, test.ShouldPanic)

	camera1, ok := client.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, _, err = camera1.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	sensorDevice, ok := client.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, err = sensorDevice.Readings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	resource1, ok := client.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, err = resource1.(arm.Arm).CurrentPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = resource1.(arm.Arm).CurrentJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToPosition(context.Background(), &commonpb.Pose{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).MoveToJointPositions(context.Background(), &componentpb.ArmJointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = resource1.(arm.Arm).JointMoveDelta(context.Background(), 0, 0)
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

	base1, ok = client.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)

	widthMillis, err := base1.WidthMillis(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, widthMillis, test.ShouldEqual, 15)

	err = base1.Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseStopCalled, test.ShouldBeTrue)
	test.That(t, capBaseName, test.ShouldEqual, "base1")

	base2, ok := client.BaseByName("base2")
	test.That(t, ok, test.ShouldBeTrue)
	err = base2.MoveStraight(context.Background(), 5, 6.2, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseMoveArgs, test.ShouldResemble, []interface{}{5, 6.2, false})
	test.That(t, capBaseName, test.ShouldEqual, "base2")

	base3, ok := client.BaseByName("base3")
	test.That(t, ok, test.ShouldBeTrue)
	err = base3.Spin(context.Background(), 7.2, 33, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseSpinArgs, test.ShouldResemble, []interface{}{7.2, 33.0, false})
	test.That(t, capBaseName, test.ShouldEqual, "base3")

	test.That(t, func() { client.RemoteByName("remote1") }, test.ShouldPanic)

	arm1, ok = client.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	pos, err := arm1.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	gripper1, ok = client.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	err = gripper1.Open(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)
	gripperOpenCalled = false

	servo1, ok = client.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	err = servo1.Move(context.Background(), 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)

	currentVal, err := servo1.AngularOffset(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, currentVal, test.ShouldEqual, 5)

	motor1, ok = client.MotorByName("motor1")
	test.That(t, motor1, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	motor2, ok := client.MotorByName("motor2")
	test.That(t, motor2, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	board1, ok = client.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	boardStatus, err := board1.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boardStatus.String(), test.ShouldResemble, status.Boards["board1"].String())

	err = board1.GPIOSet(context.Background(), "one", true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGPIOSetPin, test.ShouldEqual, "one")
	test.That(t, capGPIOSetHigh, test.ShouldBeTrue)

	isHigh, err := board1.GPIOGet(context.Background(), "one")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isHigh, test.ShouldBeTrue)
	test.That(t, capGPIOGetPin, test.ShouldEqual, "one")

	err = board1.PWMSet(context.Background(), "one", 7)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capPWMSetPin, test.ShouldEqual, "one")
	test.That(t, capPWMSetDutyCycle, test.ShouldEqual, byte(7))

	err = board1.PWMSetFreq(context.Background(), "one", 11233)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capPWMSetFreqPin, test.ShouldEqual, "one")
	test.That(t, capPWMSetFreqFreq, test.ShouldEqual, uint(11233))

	test.That(t,
		utils.NewStringSet(board1.AnalogReaderNames()...),
		test.ShouldResemble,
		utils.NewStringSet("analog1"),
	)
	test.That(t,
		utils.NewStringSet(board1.DigitalInterruptNames()...),
		test.ShouldResemble,
		utils.NewStringSet("encoder"),
	)

	board3, ok := client.BoardByName("board3")
	test.That(t, ok, test.ShouldBeTrue)
	analog1, ok = board3.AnalogReaderByName("analog1")
	test.That(t, ok, test.ShouldBeTrue)
	readVal, err := analog1.Read(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readVal, test.ShouldEqual, 6)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capAnalogReaderName, test.ShouldEqual, "analog1")

	digital1, ok = board3.DigitalInterruptByName("digital1")
	test.That(t, ok, test.ShouldBeTrue)
	digital1Config, err := digital1.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, digital1Config, test.ShouldResemble, digitalIntConfig)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

	digital1Val, err := digital1.Value(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, digital1Val, test.ShouldEqual, 287)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

	err = digital1.Tick(context.Background(), true, 44)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capDigitalInterruptHigh, test.ShouldBeTrue)
	test.That(t, capDigitalInterruptNanos, test.ShouldEqual, 44)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capDigitalInterruptName, test.ShouldEqual, "digital1")

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
	controlList, err := inputDev.Controls(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, controlList, test.ShouldResemble, []input.Control{input.AbsoluteX, input.ButtonStart})

	startTime := time.Now()
	outState, err := inputDev.LastEvents(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outState[input.ButtonStart].Event, test.ShouldEqual, input.ButtonPress)
	test.That(t, outState[input.ButtonStart].Control, test.ShouldEqual, input.ButtonStart)
	test.That(t, outState[input.ButtonStart].Value, test.ShouldEqual, 1)
	test.That(t, outState[input.ButtonStart].Time.After(startTime), test.ShouldBeTrue)
	test.That(t, outState[input.ButtonStart].Time.Before(time.Now()), test.ShouldBeTrue)

	test.That(t, outState[input.AbsoluteX].Event, test.ShouldEqual, input.PositionChangeAbs)
	test.That(t, outState[input.AbsoluteX].Control, test.ShouldEqual, input.AbsoluteX)
	test.That(t, outState[input.AbsoluteX].Value, test.ShouldEqual, 0.7)
	test.That(t, outState[input.AbsoluteX].Time.After(startTime), test.ShouldBeTrue)
	test.That(t, outState[input.AbsoluteX].Time.Before(time.Now()), test.ShouldBeTrue)

	ctrlFuncIn := func(ctx context.Context, event input.Event) { evStream <- event }
	err = inputDev.RegisterControlCallback(context.Background(), input.ButtonStart, []input.EventType{input.ButtonRelease}, ctrlFuncIn)
	test.That(t, err, test.ShouldBeNil)
	ev := <-evStream
	test.That(t, ev.Event, test.ShouldEqual, input.ButtonRelease)
	test.That(t, ev.Control, test.ShouldEqual, input.ButtonStart)
	test.That(t, ev.Value, test.ShouldEqual, 0.0)
	test.That(t, ev.Time.After(startTime), test.ShouldBeTrue)
	test.That(t, ev.Time.Before(time.Now()), test.ShouldBeTrue)
	test.That(t, capInputControllerName, test.ShouldEqual, "inputController1")

	err = inputDev.RegisterControlCallback(context.Background(), input.AbsoluteX, []input.EventType{input.PositionChangeAbs}, ctrlFuncIn)
	test.That(t, err, test.ShouldBeNil)
	ev1 := <-evStream
	ev2 := <-evStream

	var btnEv, posEv input.Event
	if ev1.Control == input.ButtonStart {
		btnEv = ev1
		posEv = ev2
	} else {
		btnEv = ev2
		posEv = ev1
	}

	test.That(t, btnEv.Event, test.ShouldEqual, input.ButtonRelease)
	test.That(t, btnEv.Control, test.ShouldEqual, input.ButtonStart)
	test.That(t, btnEv.Value, test.ShouldEqual, 0.0)
	test.That(t, btnEv.Time.After(startTime), test.ShouldBeTrue)
	test.That(t, btnEv.Time.Before(time.Now()), test.ShouldBeTrue)
	test.That(t, capInputControllerName, test.ShouldEqual, "inputController1")

	test.That(t, posEv.Event, test.ShouldEqual, input.PositionChangeAbs)
	test.That(t, posEv.Control, test.ShouldEqual, input.AbsoluteX)
	test.That(t, posEv.Value, test.ShouldEqual, 0.75)
	test.That(t, posEv.Time.After(startTime), test.ShouldBeTrue)
	test.That(t, posEv.Time.Before(time.Now()), test.ShouldBeTrue)
	test.That(t, capInputControllerName, test.ShouldEqual, "inputController1")

	err = inputDev.RegisterControlCallback(context.Background(), input.AbsoluteX, []input.EventType{input.PositionChangeAbs}, nil)
	test.That(t, err, test.ShouldBeNil)

	ev1 = <-evStream
	ev2 = <-evStream

	if ev1.Control == input.ButtonStart {
		btnEv = ev1
		posEv = ev2
	} else {
		btnEv = ev2
		posEv = ev1
	}

	test.That(t, posEv, test.ShouldResemble, input.Event{})

	test.That(t, btnEv.Event, test.ShouldEqual, input.ButtonRelease)
	test.That(t, btnEv.Control, test.ShouldEqual, input.ButtonStart)
	test.That(t, btnEv.Value, test.ShouldEqual, 0.0)
	test.That(t, btnEv.Time.After(startTime), test.ShouldBeTrue)
	test.That(t, btnEv.Time.Before(time.Now()), test.ShouldBeTrue)
	test.That(t, capInputControllerName, test.ShouldEqual, "inputController1")

	sensorDev, ok := client.SensorByName("compass1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensorDev, test.ShouldImplement, (*compass.Compass)(nil))
	test.That(t, sensorDev, test.ShouldNotImplement, (*compass.RelativeCompass)(nil))
	readings, err := sensorDev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{4.5})
	compassDev := sensorDev.(compass.Compass)
	heading, err := compassDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 4.5)
	err = compassDev.StartCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = compassDev.StopCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capSensorName, test.ShouldEqual, "compass1")

	sensorDev, ok = client.SensorByName("compass2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensorDev, test.ShouldImplement, (*compass.Compass)(nil))
	test.That(t, sensorDev, test.ShouldImplement, (*compass.RelativeCompass)(nil))
	readings, err = sensorDev.Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, []interface{}{4.5})
	compassRelDev := sensorDev.(compass.RelativeCompass)
	heading, err = compassRelDev.Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 4.5)
	err = compassRelDev.StartCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = compassRelDev.StopCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = compassRelDev.Mark(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capSensorName, test.ShouldEqual, "compass2")

	resource1, ok = client.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	pos, err = resource1.(arm.Arm).CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	forceMatrixDev, ok := client.ResourceByName(forcematrix.Named("fsm1"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, forceMatrixDev, test.ShouldImplement, (*forcematrix.ForceMatrix)(nil))
	readings, err = forceMatrixDev.(forcematrix.ForceMatrix).Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings[0], test.ShouldResemble, expectedMatrix)
	isSlipping, err := forceMatrixDev.(forcematrix.ForceMatrix).IsSlipping(context.Background())
	test.That(t, isSlipping, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)

	forceMatrixDev, ok = client.ResourceByName(forcematrix.Named("fsm2"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, forceMatrixDev, test.ShouldImplement, (*forcematrix.ForceMatrix)(nil))
	_, err = forceMatrixDev.(forcematrix.ForceMatrix).Readings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bad matrix")
	isSlipping, err = forceMatrixDev.(forcematrix.ForceMatrix).IsSlipping(context.Background())
	test.That(t, isSlipping, test.ShouldBeFalse)
	test.That(t, err, test.ShouldNotBeNil)

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
	gripperNames := []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}
	cameraNames := []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	servoNames := []resource.Name{servo.Named("servo2"), servo.Named("servo3")}
	motorNames := []resource.Name{motor.Named("motor2"), motor.Named("motor3")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(client.ArmNames()...),
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
		utils.NewStringSet("base2", "base3"),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board2", "board3"),
	)
	test.That(t,
		utils.NewStringSet(client.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet("compass2", "compass3", "compass4", "fsm1", "fsm2"),
	)
	test.That(t,
		utils.NewStringSet(client.ServoNames()...),
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
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
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
	gripperNames = []resource.Name{gripper.Named("gripper1")}
	cameraNames = []resource.Name{camera.Named("camera1")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(client.ArmNames()...),
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
		utils.NewStringSet("base1"),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board3"),
	)
	test.That(t,
		utils.NewStringSet(client.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet("compass1", "compass2", "fsm1", "fsm2"),
	)
	test.That(t,
		utils.NewStringSet(client.ServoNames()...),
		test.ShouldBeEmpty,
	)

	test.That(t, testutils.NewResourceNameSet(client.ResourceNames()...), test.ShouldResemble, testutils.NewResourceNameSet(
		testutils.ConcatResourceNames(
			armNames,
			gripperNames,
			cameraNames,
		)...))

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return finalStatus, nil
	}
	injectMetadata.AllFunc = func() []resource.Name {
		return finalResources
	}
	test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm3")}
	gripperNames = []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper3")}
	cameraNames = []resource.Name{camera.Named("camera2"), camera.Named("camera3")}
	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t,
		utils.NewStringSet(client.ArmNames()...),
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
		utils.NewStringSet("base2", "base3"),
	)
	test.That(t,
		utils.NewStringSet(client.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board2", "board3"),
	)
	test.That(t,
		utils.NewStringSet(client.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet("compass2", "compass3", "compass4", "fsm1", "fsm2"),
	)
	test.That(t,
		utils.NewStringSet(client.ServoNames()...),
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
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
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
