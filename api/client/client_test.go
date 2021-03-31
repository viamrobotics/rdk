package client

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"net"
	"testing"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot/actions"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

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

	injectRobot1.StatusFunc = func() (*pb.Status, error) {
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

	injectRobot2.StatusFunc = func() (*pb.Status, error) {
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
	injectArm.MoveToPositionFunc = func(ap *pb.ArmPosition) error {
		capArmPos = ap
		return nil
	}
	var capArmJointPos *pb.JointPositions
	injectArm.MoveToJointPositionsFunc = func(jp *pb.JointPositions) error {
		capArmJointPos = jp
		return nil
	}
	injectRobot2.ArmByNameFunc = func(name string) api.Arm {
		capArmName = name
		return injectArm
	}
	injectGripper := &inject.Gripper{}
	var gripperOpenCalled bool
	injectGripper.OpenFunc = func() error {
		gripperOpenCalled = true
		return nil
	}
	var gripperGrabCalled bool
	injectGripper.GrabFunc = func() (bool, error) {
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
	injectMotor.GoFunc = func(d pb.DirectionRelative, force byte) error {
		capGoMotorArgs = []interface{}{d, force}
		return nil
	}
	var capGoForMotorArgs []interface{}
	injectMotor.GoForFunc = func(d pb.DirectionRelative, rpm float64, rotations float64) error {
		capGoForMotorArgs = []interface{}{d, rpm, rotations}
		return nil
	}
	injectBoard.MotorFunc = func(name string) board.Motor {
		capMotorName = name
		return injectMotor
	}
	injectServo := &inject.Servo{}
	var capServoAngle uint8
	injectServo.MoveFunc = func(angle uint8) error {
		capServoAngle = angle
		return nil
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

	client, err := NewRobotClient(context.Background(), listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	_, err = client.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	stream, err := client.StatusStream(context.Background(), time.Second)
	test.That(t, err, test.ShouldBeNil)
	_, err = stream.Next()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	err = client.DoAction(context.Background(), "unknown")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")

	err = client.StopBase(context.Background(), "base1")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = client.MoveBase(context.Background(), "base1", 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = client.SpinBase(context.Background(), "base1", 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no base")

	err = client.MoveArmToPosition(context.Background(), "arm1", &pb.ArmPosition{X: 1})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.MoveArmToJointPositions(context.Background(), "arm1", &pb.JointPositions{Degrees: []float64{1}})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")

	err = client.OpenGripper(context.Background(), "gripper1")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
	_,
		err = client.GrabGripper(context.Background(), "gripper1")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")

	err = client.GoMotor(context.Background(), "board1", "motor1", pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 0, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")

	err = client.MoveServo(context.Background(), "board1", "servo1", 5)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
	_,
		err = client.CameraFrame(context.Background(), "camera1")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)

	// working
	client, err = NewRobotClient(context.Background(), listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)

	status, err := client.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status.String(), test.ShouldResemble, emptyStatus.String())

	dur := 100 * time.Millisecond
	start := time.Now()
	stream, err = client.StatusStream(context.Background(), dur)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 3; i++ {
		nextStatus, err := stream.Next()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, nextStatus.String(), test.ShouldResemble, emptyStatus.String())
	}
	test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
	test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 6*dur)

	actionName := utils.RandomAlphaString(5)
	called := make(chan api.Robot)
	actions.RegisterAction(actionName, func(r api.Robot) {
		called <- r
	})

	err = client.DoAction(context.Background(), actionName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, <-called, test.ShouldEqual, injectRobot2)

	err = client.StopBase(context.Background(), "base1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, baseStopCalled, test.ShouldBeTrue)
	test.That(t, capBaseName, test.ShouldEqual, "base1")

	err = client.MoveBase(context.Background(), "base2", 5, 6.2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseMoveArgs, test.ShouldResemble, []interface{}{5, 6.2, false})
	test.That(t, capBaseName, test.ShouldEqual, "base2")

	err = client.SpinBase(context.Background(), "base3", 7.2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capBaseSpinArgs, test.ShouldResemble, []interface{}{7.2, 64, false})
	test.That(t, capBaseName, test.ShouldEqual, "base3")

	pos := &pb.ArmPosition{X: 1, Y: 2, Z: 3, RX: 4, RY: 5, RZ: 6}
	err = client.MoveArmToPosition(context.Background(), "arm1", pos)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capArmPos.String(), test.ShouldResemble, pos.String())
	test.That(t, capArmName, test.ShouldEqual, "arm1")

	jointPos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
	err = client.MoveArmToJointPositions(context.Background(), "arm2", jointPos)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capArmJointPos.String(), test.ShouldResemble, jointPos.String())
	test.That(t, capArmName, test.ShouldEqual, "arm2")

	err = client.OpenGripper(context.Background(), "gripper1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gripperOpenCalled, test.ShouldBeTrue)
	test.That(t, gripperGrabCalled, test.ShouldBeFalse)
	test.That(t, capGripperName, test.ShouldEqual, "gripper1")
	gripperOpenCalled = false

	grabbed, err := client.GrabGripper(context.Background(), "gripper2")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, grabbed, test.ShouldBeTrue)
	test.That(t, gripperOpenCalled, test.ShouldBeFalse)
	test.That(t, gripperGrabCalled, test.ShouldBeTrue)
	test.That(t, capGripperName, test.ShouldEqual, "gripper2")

	err = client.GoMotor(context.Background(), "board1", "motor1", pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1.2, 3.4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGoForMotorArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1.2, 3.4})
	test.That(t, capBoardName, test.ShouldEqual, "board1")
	test.That(t, capMotorName, test.ShouldEqual, "motor1")

	err = client.GoMotor(context.Background(), "board2", "motor2", pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 1.2, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capGoMotorArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, byte(1)})
	test.That(t, capBoardName, test.ShouldEqual, "board2")
	test.That(t, capMotorName, test.ShouldEqual, "motor2")

	err = client.MoveServo(context.Background(), "board3", "servo1", 4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capServoAngle, test.ShouldEqual, 4)
	test.That(t, capBoardName, test.ShouldEqual, "board3")
	test.That(t, capServoName, test.ShouldEqual, "servo1")

	frame, err := client.CameraFrame(context.Background(), "camera1")
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 262140)
	test.That(t, imageReleased, test.ShouldBeTrue)
	test.That(t, capCameraName, test.ShouldEqual, "camera1")

	err = client.Close()
	test.That(t, err, test.ShouldBeNil)
}
