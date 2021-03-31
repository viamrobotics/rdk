package server_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/robotcore/api"
	apiserver "go.viam.com/robotcore/api/server"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot/actions"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

func newServer() (pb.RobotServiceServer, *inject.Robot) {
	injectRobot := &inject.Robot{}
	return apiserver.New(injectRobot), injectRobot
}

var emptyStatus = &pb.StatusResponse{
	Status: &pb.Status{
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
	},
}

func TestServer(t *testing.T) {
	t.Run("Status", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func() (*pb.Status, error) {
			return nil, err1
		}
		_, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldEqual, err1)

		injectRobot.StatusFunc = func() (*pb.Status, error) {
			return emptyStatus.Status, nil
		}
		statusResp, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusResp, test.ShouldResemble, emptyStatus)
	})

	t.Run("StatusStream", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func() (*pb.Status, error) {
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

		injectRobot.StatusFunc = func() (*pb.Status, error) {
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
		go func() {
			streamErr = server.StatusStream(&pb.StatusStreamRequest{
				Every: durationpb.New(dur),
			}, streamServer)
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
		test.That(t, streamErr, test.ShouldBeNil)

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
		called := make(chan api.Robot)
		actions.RegisterAction(actionName, func(r api.Robot) {
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)
	})

	t.Run("ControlBase", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BaseByNameFunc = func(name string) api.Base {
			capName = name
			return nil
		}

		_, err := server.ControlBase(context.Background(), &pb.ControlBaseRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		injectBase := &inject.Base{}
		injectRobot.BaseByNameFunc = func(name string) api.Base {
			return injectBase
		}
		_, err = server.ControlBase(context.Background(), &pb.ControlBaseRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown action")

		var capCtx context.Context
		err1 := errors.New("whoops")
		injectBase.StopFunc = func(ctx context.Context) error {
			capCtx = ctx
			return err1
		}

		ctx := context.Background()
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Stop{
				Stop: false,
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capCtx, test.ShouldBeNil)

		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Stop{
				Stop: true,
			},
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectBase.StopFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Stop{
				Stop: true,
			},
		})
		test.That(t, err, test.ShouldBeNil)

		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name:   "base1",
			Action: &pb.ControlBaseRequest_Move{},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unspecified")

		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Move{
				Move: &pb.MoveBase{},
			},
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown move")

		var capArgs []interface{}
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return err1
		}
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Move{
				Move: &pb.MoveBase{
					Option: &pb.MoveBase_StraightDistanceMillis{
						StraightDistanceMillis: 1,
					},
				},
			},
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 500.0, false})

		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return nil
		}
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Move{
				Move: &pb.MoveBase{
					Speed: 2.3,
					Option: &pb.MoveBase_StraightDistanceMillis{
						StraightDistanceMillis: 1,
					},
				},
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 2.3, false})

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			capArgs = []interface{}{ctx, angleDeg, speed, block}
			return err1
		}
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Move{
				Move: &pb.MoveBase{
					Option: &pb.MoveBase_SpinAngleDeg{
						SpinAngleDeg: 4.5,
					},
				},
			},
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 64, false})

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, speed int, block bool) error {
			capArgs = []interface{}{ctx, angleDeg, speed, block}
			return nil
		}
		_, err = server.ControlBase(ctx, &pb.ControlBaseRequest{
			Name: "base1",
			Action: &pb.ControlBaseRequest_Move{
				Move: &pb.MoveBase{
					Speed: 20.3,
					Option: &pb.MoveBase_SpinAngleDeg{
						SpinAngleDeg: 4.5,
					},
				},
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 64, false})
	})

	t.Run("MoveArmToPosition", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) api.Arm {
			capName = name
			return nil
		}

		_, err := server.MoveArmToPosition(context.Background(), &pb.MoveArmToPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) api.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		var capAP *pb.ArmPosition
		injectArm.MoveToPositionFunc = func(ap *pb.ArmPosition) error {
			capAP = ap
			return err1
		}

		pos := &pb.ArmPosition{X: 1, Y: 2, Z: 3, RX: 4, RY: 5, RZ: 6}
		_, err = server.MoveArmToPosition(context.Background(), &pb.MoveArmToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAP, test.ShouldEqual, pos)

		injectArm.MoveToPositionFunc = func(ap *pb.ArmPosition) error {
			return nil
		}
		_, err = server.MoveArmToPosition(context.Background(), &pb.MoveArmToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("MoveArmToJointPositions", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) api.Arm {
			capName = name
			return nil
		}

		_, err := server.MoveArmToJointPositions(context.Background(), &pb.MoveArmToJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) api.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		var capJP *pb.JointPositions
		injectArm.MoveToJointPositionsFunc = func(jp *pb.JointPositions) error {
			capJP = jp
			return err1
		}

		pos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
		_, err = server.MoveArmToJointPositions(context.Background(), &pb.MoveArmToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capJP, test.ShouldEqual, pos)

		injectArm.MoveToJointPositionsFunc = func(jp *pb.JointPositions) error {
			return nil
		}
		_, err = server.MoveArmToJointPositions(context.Background(), &pb.MoveArmToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ControlGripper", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.GripperByNameFunc = func(name string) api.Gripper {
			capName = name
			return nil
		}

		_, err := server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
		test.That(t, capName, test.ShouldEqual, "gripper1")

		injectGripper := &inject.Gripper{}
		injectRobot.GripperByNameFunc = func(name string) api.Gripper {
			return injectGripper
		}
		_, err = server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown action")

		err1 := errors.New("whoops")
		injectGripper.OpenFunc = func() error {
			return err1
		}
		_, err = server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name:   "gripper1",
			Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN,
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.OpenFunc = func() error {
			return nil
		}
		resp, err := server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name:   "gripper1",
			Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN,
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeFalse)

		injectGripper.GrabFunc = func() (bool, error) {
			return false, err1
		}
		_, err = server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name:   "gripper1",
			Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB,
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.GrabFunc = func() (bool, error) {
			return false, nil
		}

		resp, err = server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name:   "gripper1",
			Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB,
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeFalse)

		injectGripper.GrabFunc = func() (bool, error) {
			return true, nil
		}
		resp, err = server.ControlGripper(context.Background(), &pb.ControlGripperRequest{
			Name:   "gripper1",
			Action: pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB,
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeTrue)
	})

	t.Run("ControlBoardMotor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			capName = name
			return nil
		}

		_, err := server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			return injectBoard
		}
		injectBoard.MotorFunc = func(name string) board.Motor {
			capName = name
			return nil
		}

		_, err = server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
			MotorName: "motor1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown motor")
		test.That(t, capName, test.ShouldEqual, "motor1")

		injectMotor := &inject.Motor{}
		injectBoard.MotorFunc = func(name string) board.Motor {
			return injectMotor
		}

		var capArgs []interface{}
		err1 := errors.New("whoops")
		injectMotor.GoFunc = func(d pb.DirectionRelative, force byte) error {
			capArgs = []interface{}{d, force}
			return err1
		}
		_, err = server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
			MotorName: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, byte(0)})

		injectMotor.GoFunc = func(d pb.DirectionRelative, force byte) error {
			capArgs = []interface{}{d, force}
			return nil
		}
		_, err = server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
			MotorName: "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD,
			Speed:     2.3,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, byte(2)})

		injectMotor.GoFunc = func(d pb.DirectionRelative, force byte) error {
			return errors.New("no")
		}
		injectMotor.GoForFunc = func(d pb.DirectionRelative, rpm float64, rotations float64) error {
			capArgs = []interface{}{d, rpm, rotations}
			return err1
		}
		_, err = server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
			MotorName: "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Speed:     2.3,
			Rotations: 4.5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})

		injectMotor.GoForFunc = func(d pb.DirectionRelative, rpm float64, rotations float64) error {
			capArgs = []interface{}{d, rpm, rotations}
			return nil
		}
		_, err = server.ControlBoardMotor(context.Background(), &pb.ControlBoardMotorRequest{
			BoardName: "board1",
			MotorName: "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Speed:     2.3,
			Rotations: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})
	})

	t.Run("ControlBoardServo", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			capName = name
			return nil
		}

		_, err := server.ControlBoardServo(context.Background(), &pb.ControlBoardServoRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			return injectBoard
		}
		injectBoard.ServoFunc = func(name string) board.Servo {
			capName = name
			return nil
		}

		_, err = server.ControlBoardServo(context.Background(), &pb.ControlBoardServoRequest{
			BoardName: "board1",
			ServoName: "servo1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown servo")
		test.That(t, capName, test.ShouldEqual, "servo1")

		injectServo := &inject.Servo{}
		injectBoard.ServoFunc = func(name string) board.Servo {
			return injectServo
		}

		var capAngle uint8
		err1 := errors.New("whoops")
		injectServo.MoveFunc = func(angle uint8) error {
			capAngle = angle
			return err1
		}
		_, err = server.ControlBoardServo(context.Background(), &pb.ControlBoardServoRequest{
			BoardName: "board1",
			ServoName: "servo1",
			AngleDeg:  5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAngle, test.ShouldEqual, 5)

		injectServo.MoveFunc = func(angle uint8) error {
			capAngle = angle
			return nil
		}
		_, err = server.ControlBoardServo(context.Background(), &pb.ControlBoardServoRequest{
			BoardName: "board1",
			ServoName: "servo1",
			AngleDeg:  5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capAngle, test.ShouldEqual, 5)
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
