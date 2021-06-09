package client

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"net"
	"testing"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/grpc/server"
	"go.viam.com/core/lidar"
	"go.viam.com/core/pointcloud"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rpc/dialer"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/compass"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

var emptyStatus = &pb.Status{
	Arms: map[string]*pb.ArmStatus{
		"arm1": {
			GridPosition: &pb.ArmPosition{
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
	Sensors: map[string]*pb.SensorStatus{
		"compass1": {
			Type: compass.Type,
		},
		"compass2": {
			Type: compass.RelativeType,
		},
	},
	Boards: map[string]*pb.BoardStatus{
		"board1": {
			Motors: map[string]*pb.MotorStatus{
				"g": {},
			},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
			},
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
		"board2": {
			Motors: map[string]*pb.MotorStatus{
				"motor2": {},
			},
		},
		"board3": {
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
			},
		},
	},
}

var finalStatus = &pb.Status{
	Arms: map[string]*pb.ArmStatus{
		"arm2": {
			GridPosition: &pb.ArmPosition{
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
			GridPosition: &pb.ArmPosition{
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
	Lidars: map[string]bool{
		"lidar2": true,
		"lidar3": true,
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
	},
	Boards: map[string]*pb.BoardStatus{
		"board2": {
			Motors: map[string]*pb.MotorStatus{
				"g": {},
			},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
			},
			Analogs: map[string]*pb.AnalogStatus{
				"analog1": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
		"board3": {
			Motors: map[string]*pb.MotorStatus{
				"g": {},
			},
			Servos: map[string]*pb.ServoStatus{
				"servo2": {},
			},
			Analogs: map[string]*pb.AnalogStatus{
				"analog2": {},
			},
			DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{
				"encoder": {},
			},
		},
	},
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
	injectRobot1.BaseByNameFunc = func(name string) base.Base {
		return nil
	}
	injectRobot1.ArmByNameFunc = func(name string) arm.Arm {
		return nil
	}
	injectRobot1.GripperByNameFunc = func(name string) gripper.Gripper {
		return nil
	}
	injectRobot1.BoardByNameFunc = func(name string) board.Board {
		return nil
	}
	injectRobot1.CameraByNameFunc = func(name string) camera.Camera {
		return nil
	}
	injectRobot1.LidarByNameFunc = func(name string) lidar.Lidar {
		return nil
	}
	injectRobot1.SensorByNameFunc = func(name string) sensor.Sensor {
		return nil
	}

	injectRobot2.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}
	var (
		capBaseName    string
		capArmName     string
		capGripperName string
		capBoardName   string
		capMotorName   string
		capServoName   string
		capCameraName  string
		capLidarName   string
		capSensorName  string
	)
	injectBase := &inject.Base{}
	var baseStopCalled bool
	injectBase.StopFunc = func(ctx context.Context) error {
		baseStopCalled = true
		return nil
	}
	var capBaseMoveArgs []interface{}
	injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
		capBaseMoveArgs = []interface{}{distanceMillis, millisPerSec, block}
		return distanceMillis, nil
	}
	var capBaseSpinArgs []interface{}
	injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
		capBaseSpinArgs = []interface{}{angleDeg, degsPerSec, block}
		return angleDeg, nil
	}
	injectRobot2.BaseByNameFunc = func(name string) base.Base {
		capBaseName = name
		return injectBase
	}
	injectArm := &inject.Arm{}
	var capArmPos *pb.ArmPosition
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*pb.ArmPosition, error) {
		return emptyStatus.Arms["arm1"].GridPosition, nil
	}
	injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return emptyStatus.Arms["arm1"].JointPositions, nil
	}
	injectArm.MoveToPositionFunc = func(ctx context.Context, ap *pb.ArmPosition) error {
		capArmPos = ap
		return nil
	}
	var capArmJointPos *pb.JointPositions
	injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions) error {
		capArmJointPos = jp
		return nil
	}
	injectRobot2.ArmByNameFunc = func(name string) arm.Arm {
		capArmName = name
		return injectArm
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
	injectRobot2.GripperByNameFunc = func(name string) gripper.Gripper {
		capGripperName = name
		return injectGripper
	}
	injectBoard := &inject.Board{}
	injectMotor := &inject.Motor{}
	var capGoMotorArgs []interface{}
	injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
		capGoMotorArgs = []interface{}{d, powerPct}
		return nil
	}
	var capGoForMotorArgs []interface{}
	injectMotor.GoForFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
		capGoForMotorArgs = []interface{}{d, rpm, rotations}
		return nil
	}
	injectBoard.MotorFunc = func(name string) board.Motor {
		capMotorName = name
		return injectMotor
	}
	injectServo := &inject.Servo{}
	var capServoAngle uint8
	injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		capServoAngle = angle
		return nil
	}
	injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
		return emptyStatus.Boards["board1"], nil
	}
	injectBoard.ServoFunc = func(name string) board.Servo {
		capServoName = name
		return injectServo
	}
	injectRobot2.BoardByNameFunc = func(name string) board.Board {
		capBoardName = name
		return injectBoard
	}
	injectCamera := &inject.Camera{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, jpeg.Encode(&imgBuf, img, nil), test.ShouldBeNil)

	pcA := pointcloud.New()
	err = pcA.Set(pointcloud.NewBasicPoint(5, 5, 5))
	test.That(t, err, test.ShouldBeNil)

	var imageReleased bool
	injectCamera.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return img, func() { imageReleased = true }, nil
	}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pcA, nil
	}

	injectRobot2.CameraByNameFunc = func(name string) camera.Camera {
		capCameraName = name
		return injectCamera
	}

	injectLidarDev := &inject.Lidar{}
	injectLidarDev.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"hello": "world"}, nil
	}
	injectLidarDev.StartFunc = func(ctx context.Context) error {
		return nil
	}
	injectLidarDev.StopFunc = func(ctx context.Context) error {
		return nil
	}
	injectLidarDev.CloseFunc = func() error {
		return nil
	}
	injectLidarDev.ScanFunc = func(ctx context.Context, opts lidar.ScanOptions) (lidar.Measurements, error) {
		return lidar.Measurements{lidar.NewMeasurement(2, 40)}, nil
	}
	injectLidarDev.RangeFunc = func(ctx context.Context) (float64, error) {
		return 25, nil
	}
	injectLidarDev.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
		return r2.Point{4, 5}, nil
	}
	injectLidarDev.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 5.2, nil
	}
	injectRobot2.LidarByNameFunc = func(name string) lidar.Lidar {
		capLidarName = name
		return injectLidarDev
	}

	injectCompassDev := &inject.Compass{}
	injectRelCompassDev := &inject.RelativeCompass{}
	injectRobot2.SensorByNameFunc = func(name string) sensor.Sensor {
		capSensorName = name
		if name == "compass2" {
			return injectRelCompassDev
		}
		return injectCompassDev
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

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	// failing
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewClient(cancelCtx, listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{
			Boards: map[string]*pb.BoardStatus{
				"board1": {},
				"board2": {
					Motors: map[string]*pb.MotorStatus{
						"motor1": {},
					},
				},
			},
		}, nil
	}

	cfg := config.Config{
		Components: []config.Component{
			{
				Name:   "a",
				Type:   config.ComponentTypeArm,
				Parent: "b",
				ParentTranslation: config.Translation{
					X: 1,
					Y: 2,
					Z: 3,
				},
				ParentOrientation: config.Orientation{
					X:  4,
					Y:  5,
					Z:  6,
					TH: 7,
				},
			},
		},
	}
	injectRobot1.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return &cfg, nil
	}

	client, err := NewClient(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	newCfg, err := client.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newCfg.Components[0], test.ShouldResemble, cfg.Components[0])

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return nil, errors.New("whoops")
	}
	_, err = client.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	err = client.BaseByName("base1").Stop(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	_, err = client.BaseByName("base1").MoveStraight(context.Background(), 5, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	_, err = client.BaseByName("base1").Spin(context.Background(), 5.2, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	_, err = client.BaseByName("base1").WidthMillis(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)

	_, err = client.ArmByName("arm1").CurrentPosition(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	_, err = client.ArmByName("arm1").CurrentJointPositions(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.ArmByName("arm1").MoveToPosition(context.Background(), &pb.ArmPosition{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.ArmByName("arm1").MoveToJointPositions(context.Background(), &pb.JointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.ArmByName("arm1").JointMoveDelta(context.Background(), 0, 0)
	test.That(t, err, test.ShouldEqual, errUnimplemented)

	err = client.GripperByName("gripper1").Open(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_, err = client.GripperByName("gripper1").Grab(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	board1 := client.BoardByName("board1")
	test.That(t, board1, test.ShouldNotBeNil)

	test.That(t, board1.ModelAttributes(), test.ShouldResemble, board.ModelAttributes{Remote: true})

	test.That(t, client.BoardByName("boardwhat"), test.ShouldBeNil)

	_, err = board1.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.Motor("motor1").Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	err = board1.Motor("motor1").GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.Motor("motor1").Power(context.Background(), 0)
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	_, err = board1.Motor("motor1").Position(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	_, err = board1.Motor("motor1").PositionSupported(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	err = board1.Motor("motor1").Off(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	_, err = board1.Motor("motor1").IsOn(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)

	err = board1.Servo("servo1").Move(context.Background(), 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	_, err = board1.Servo("servo1").Current(context.Background())
	test.That(t, err, test.ShouldEqual, errUnimplemented)

	test.That(t, func() { board1.AnalogReader("analog1").Read(context.Background()) }, test.ShouldPanic)
	test.That(t, func() { board1.DigitalInterrupt("digital1").Config() }, test.ShouldPanic)
	test.That(t, func() { board1.DigitalInterrupt("digital1").Value() }, test.ShouldPanic)
	test.That(t, func() { board1.DigitalInterrupt("digital1").Tick(true, 0) }, test.ShouldPanic)
	test.That(t, func() { board1.DigitalInterrupt("digital1").AddCallback(nil) }, test.ShouldPanic)
	test.That(t, func() { board1.DigitalInterrupt("digital1").AddPostProcessor(nil) }, test.ShouldPanic)

	_, _, err = client.CameraByName("camera1").Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	sensorDevice := client.SensorByName("sensor1")
	_, err = sensorDevice.Readings(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)

	// working
	client, err = NewClient(context.Background(), listener2.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, emptyStatus.String())

	err = client.BaseByName("base1").Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseStopCalled, test.ShouldBeTrue)
	test.That(t, capBaseName, test.ShouldEqual, "base1")

	moved, err := client.BaseByName("base2").MoveStraight(context.Background(), 5, 6.2, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseMoveArgs, test.ShouldResemble, []interface{}{5, 6.2, false})
	test.That(t, capBaseName, test.ShouldEqual, "base2")
	test.That(t, moved, test.ShouldEqual, 5)

	spun, err := client.BaseByName("base3").Spin(context.Background(), 7.2, 33, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseSpinArgs, test.ShouldResemble, []interface{}{7.2, 33.0, false})
	test.That(t, capBaseName, test.ShouldEqual, "base3")
	test.That(t, spun, test.ShouldEqual, 7.2)

	test.That(t, func() { client.RemoteByName("remote1") }, test.ShouldPanic)

	pos, err := client.ArmByName("arm1").CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	jp, err := client.ArmByName("arm1").CurrentJointPositions(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jp.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].JointPositions.String())

	pos = &pb.ArmPosition{X: 1, Y: 2, Z: 3, OX: 4, OY: 5, OZ: 6}
	err = client.ArmByName("arm1").MoveToPosition(context.Background(), pos)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capArmPos.String(), test.ShouldResemble, pos.String())
	test.That(t, capArmName, test.ShouldEqual, "arm1")

	jointPos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
	err = client.ArmByName("arm2").MoveToJointPositions(context.Background(), jointPos)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos.String())
	test.That(t, capArmName, test.ShouldEqual, "arm2")

	err = client.GripperByName("gripper1").Open(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)
	test.That(t, capGripperName, test.ShouldEqual, "gripper1")
	gripperOpenCalled = false

	grabbed, err := client.GripperByName("gripper2").Grab(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, grabbed, test.ShouldBeTrue)
	test.That(t, gripperOpenCalled, test.ShouldBeFalse)
	test.That(t, gripperGrabCalled, test.ShouldBeTrue)
	test.That(t, capGripperName, test.ShouldEqual, "gripper2")

	board1 = client.BoardByName("board1")
	boardStatus, err := board1.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boardStatus.String(), test.ShouldResemble, status.Boards["board1"].String())

	test.That(t, utils.NewStringSet(board1.MotorNames()...), test.ShouldResemble, utils.NewStringSet("g"))
	test.That(t, utils.NewStringSet(board1.ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo1"))
	test.That(t, utils.NewStringSet(board1.AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog1"))
	test.That(t, utils.NewStringSet(board1.DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("encoder"))

	err = board1.Motor("motor1").Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGoMotorArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(1)})
	test.That(t, capBoardName, test.ShouldEqual, "board1")
	test.That(t, capMotorName, test.ShouldEqual, "motor1")

	err = client.BoardByName("board2").Motor("motor2").GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1.2, 3.4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGoForMotorArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1.2, 3.4})
	test.That(t, capBoardName, test.ShouldEqual, "board2")
	test.That(t, capMotorName, test.ShouldEqual, "motor2")

	err = client.BoardByName("board3").Servo("servo1").Move(context.Background(), 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capServoName, test.ShouldEqual, "servo1")

	frame, _, err := client.CameraByName("camera1").Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0) // exact copy, no color conversion
	test.That(t, imageReleased, test.ShouldBeTrue)
	test.That(t, capCameraName, test.ShouldEqual, "camera1")

	pcB, err := client.CameraByName("camera1").NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcB.At(5, 5, 5), test.ShouldNotBeNil)

	lidarDev := client.LidarByName("lidar1")
	info, err := lidarDev.Info(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info, test.ShouldResemble, map[string]interface{}{"hello": "world"})
	err = lidarDev.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = lidarDev.Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	scan, err := lidarDev.Scan(context.Background(), lidar.ScanOptions{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, scan, test.ShouldResemble, lidar.Measurements{lidar.NewMeasurement(2, 40)})
	devRange, err := lidarDev.Range(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, devRange, test.ShouldEqual, 25)
	bounds, err := lidarDev.Bounds(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bounds, test.ShouldResemble, r2.Point{4, 5})
	angRes, err := lidarDev.AngularResolution(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angRes, test.ShouldEqual, 5.2)
	err = utils.TryClose(lidarDev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capLidarName, test.ShouldEqual, "lidar1")

	sensorDev := client.SensorByName("compass1")
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

	sensorDev = client.SensorByName("compass2")
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

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)
}

func TestClientReferesh(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

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

	start := time.Now()
	dur := 100 * time.Millisecond
	client, err := NewClientWithOptions(
		context.Background(),
		listener.Addr().String(),
		RobotClientOptions{RefreshEvery: dur, Insecure: true},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	<-calledEnough
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 5*dur)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 10*dur)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, finalStatus.String())

	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(client.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm2", "arm3"))
	test.That(t, utils.NewStringSet(client.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper2", "gripper3"))
	test.That(t, utils.NewStringSet(client.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera2", "camera3"))
	test.That(t, utils.NewStringSet(client.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2", "lidar3"))
	test.That(t, utils.NewStringSet(client.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2", "base3"))
	test.That(t, utils.NewStringSet(client.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2", "board3"))
	test.That(t, utils.NewStringSet(client.SensorNames()...), test.ShouldResemble, utils.NewStringSet("compass2", "compass3", "compass4"))

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}
	client, err = NewClientWithOptions(
		context.Background(),
		listener.Addr().String(),
		RobotClientOptions{RefreshEvery: dur, Insecure: true},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(client.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
	test.That(t, utils.NewStringSet(client.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1"))
	test.That(t, utils.NewStringSet(client.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1"))
	test.That(t, utils.NewStringSet(client.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1"))
	test.That(t, utils.NewStringSet(client.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
	test.That(t, utils.NewStringSet(client.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board3"))
	test.That(t, utils.NewStringSet(client.SensorNames()...), test.ShouldResemble, utils.NewStringSet("compass1", "compass2"))

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return finalStatus, nil
	}
	test.That(t, client.Refresh(context.Background()), test.ShouldBeNil)

	test.That(t, client.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(client.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm2", "arm3"))
	test.That(t, utils.NewStringSet(client.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper2", "gripper3"))
	test.That(t, utils.NewStringSet(client.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera2", "camera3"))
	test.That(t, utils.NewStringSet(client.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2", "lidar3"))
	test.That(t, utils.NewStringSet(client.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2", "base3"))
	test.That(t, utils.NewStringSet(client.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2", "board3"))
	test.That(t, utils.NewStringSet(client.SensorNames()...), test.ShouldResemble, utils.NewStringSet("compass2", "compass3", "compass4"))

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectRobot := &inject.Robot{}
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return emptyStatus, nil
	}

	td := &trackingDialer{Dialer: dialer.NewCachedDialer()}
	ctx := dialer.ContextWithDialer(context.Background(), td)
	client1, err := NewClient(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2, err := NewClient(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.dialCalled, test.ShouldEqual, 2)

	err = client1.Close()
	test.That(t, err, test.ShouldBeNil)
	err = client2.Close()
	test.That(t, err, test.ShouldBeNil)
}

type trackingDialer struct {
	dialer.Dialer
	dialCalled int
}

func (td *trackingDialer) Dial(ctx context.Context, target string, opts ...grpc.DialOption) (dialer.ClientConn, error) {
	td.dialCalled++
	return td.Dialer.Dial(ctx, target, opts...)
}
