package client

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/gostream"
	"github.com/edaniels/test"
	"google.golang.org/grpc"
)

var emptyStatus = &pb.Status{
	Arms: map[string]*pb.ArmStatus{
		"arm1": {
			GridPosition: &pb.ArmPosition{
				X:  0.0,
				Y:  0.0,
				Z:  0.0,
				RX: 0.0,
				RY: 0.0,
				RZ: 0.0,
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
	},
}

func TestClient(t *testing.T) {
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
	injectRobot1.BaseByNameFunc = func(name string) api.Base {
		return nil
	}
	injectRobot1.ArmByNameFunc = func(name string) api.Arm {
		return nil
	}
	injectRobot1.GripperByNameFunc = func(name string) api.Gripper {
		return nil
	}
	injectRobot1.BoardByNameFunc = func(name string) board.Board {
		return nil
	}
	injectRobot1.CameraByNameFunc = func(name string) gostream.ImageSource {
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
	)
	injectBase := &inject.Base{}
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
	injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
		capBaseSpinArgs = []interface{}{angleDeg, speed, block}
		return nil
	}
	injectRobot2.BaseByNameFunc = func(name string) api.Base {
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
	injectRobot2.ArmByNameFunc = func(name string) api.Arm {
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
	injectRobot2.GripperByNameFunc = func(name string) api.Gripper {
		capGripperName = name
		return injectGripper
	}
	injectBoard := &inject.Board{}
	injectMotor := &inject.Motor{}
	var capGoMotorArgs []interface{}
	injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, force byte) error {
		capGoMotorArgs = []interface{}{d, force}
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
	injectImageSource := &inject.ImageSource{}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, jpeg.Encode(&imgBuf, img, nil), test.ShouldBeNil)

	var imageReleased bool
	injectImageSource.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
		return img, func() { imageReleased = true }, nil
	}
	injectRobot2.CameraByNameFunc = func(name string) gostream.ImageSource {
		capCameraName = name
		return injectImageSource
	}

	go gServer1.Serve(listener1)
	defer gServer1.Stop()
	go gServer2.Serve(listener2)
	defer gServer2.Stop()

	// failing
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewRobotClient(cancelCtx, listener1.Addr().String())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return &pb.Status{}, nil
	}
	client, err := NewRobotClient(context.Background(), listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)

	injectRobot1.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return nil, errors.New("whoops")
	}
	_, err = client.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	err = client.BaseByName("base1").Stop(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = client.BaseByName("base1").MoveStraight(context.Background(), 0, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = client.BaseByName("base1").Spin(context.Background(), 0, 0, false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

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

	err = client.GripperByName("gripper1").Open(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_, err = client.GripperByName("gripper1").Grab(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	board1 := client.BoardByName("board1")

	_, err = board1.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.Motor("motor1").Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	err = board1.Motor("motor1").GoFor(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = board1.Servo("servo1").Move(context.Background(), 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	_, _, err = client.CameraByName("camera1").Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// working
	client, err = NewRobotClient(context.Background(), listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, emptyStatus.String())

	err = client.BaseByName("base1").Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseStopCalled, test.ShouldBeTrue)
	test.That(t, capBaseName, test.ShouldEqual, "base1")

	err = client.BaseByName("base2").MoveStraight(context.Background(), 5, 6.2, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseMoveArgs, test.ShouldResemble, []interface{}{5, 6.2, false})
	test.That(t, capBaseName, test.ShouldEqual, "base2")

	err = client.BaseByName("base3").Spin(context.Background(), 7.2, 33, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseSpinArgs, test.ShouldResemble, []interface{}{7.2, 64, false}) // 64 is hardcoded
	test.That(t, capBaseName, test.ShouldEqual, "base3")

	pos, err := client.ArmByName("arm1").CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].GridPosition.String())

	jp, err := client.ArmByName("arm1").CurrentJointPositions(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jp.String(), test.ShouldResemble, emptyStatus.Arms["arm1"].JointPositions.String())

	pos = &pb.ArmPosition{X: 1, Y: 2, Z: 3, RX: 4, RY: 5, RZ: 6}
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

	boardStatus, err := client.BoardByName("board1").Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boardStatus.String(), test.ShouldResemble, status.Boards["board1"].String())

	err = client.BoardByName("board1").Motor("motor1").Go(context.Background(), pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGoMotorArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, uint8(1)})
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
	test.That(t, compVal, test.ShouldEqual, 262140)
	test.That(t, imageReleased, test.ShouldBeTrue)
	test.That(t, capCameraName, test.ShouldEqual, "camera1")

	err = client.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
