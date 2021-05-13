package server_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"math"
	"testing"
	"time"

	"go.viam.com/robotcore/action"
	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/base"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/grpc/client"
	apiserver "go.viam.com/robotcore/grpc/server"
	"go.viam.com/robotcore/lidar"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/gostream"
	"github.com/golang/geo/r2"
	"go.viam.com/test"
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
		Cameras: map[string]bool{
			"camera1": true,
		},
		Lidars: map[string]bool{
			"lidar1": true,
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
		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return nil, err1
		}
		_, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldEqual, err1)

		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
			return emptyStatus.Status, nil
		}
		statusResp, err := server.Status(context.Background(), &pb.StatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusResp, test.ShouldResemble, emptyStatus)
	})

	t.Run("StatusStream", func(t *testing.T) {
		server, injectRobot := newServer()
		err1 := errors.New("whoops")
		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
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

		injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
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
		done := make(chan struct{})
		go func() {
			streamErr = server.StatusStream(&pb.StatusStreamRequest{
				Every: durationpb.New(dur),
			}, streamServer)
			close(done)
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
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)

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
		called := make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)

		actionName = utils.RandomAlphaString(5)
		called = make(chan robot.Robot)
		action.RegisterAction(actionName, func(ctx context.Context, r robot.Robot) {
			go utils.TryClose(server)
			<-ctx.Done()
			called <- r
		})

		_, err = server.DoAction(context.Background(), &pb.DoActionRequest{
			Name: actionName,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, <-called, test.ShouldEqual, injectRobot)
	})

	t.Run("Base", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BaseByNameFunc = func(name string) base.Base {
			capName = name
			return nil
		}

		_, err := server.BaseMoveStraight(context.Background(), &pb.BaseMoveStraightRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		_, err = server.BaseSpin(context.Background(), &pb.BaseSpinRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		_, err = server.BaseStop(context.Background(), &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no base")
		test.That(t, capName, test.ShouldEqual, "base1")

		injectBase := &inject.Base{}
		injectRobot.BaseByNameFunc = func(name string) base.Base {
			return injectBase
		}
		var capCtx context.Context
		err1 := errors.New("whoops")
		injectBase.StopFunc = func(ctx context.Context) error {
			capCtx = ctx
			return err1
		}

		ctx := context.Background()
		_, err = server.BaseStop(ctx, &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capCtx, test.ShouldEqual, ctx)

		injectBase.StopFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.BaseStop(ctx, &pb.BaseStopRequest{
			Name: "base1",
		})
		test.That(t, err, test.ShouldBeNil)

		var capArgs []interface{}
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return 2, err1
		}
		resp, err := server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 500.0, false})
		test.That(t, resp.Success, test.ShouldBeFalse)
		test.That(t, resp.Error, test.ShouldEqual, err1.Error())
		test.That(t, resp.DistanceMillis, test.ShouldEqual, 2)

		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
			capArgs = []interface{}{ctx, distanceMillis, millisPerSec, block}
			return distanceMillis, nil
		}
		resp, err = server.BaseMoveStraight(ctx, &pb.BaseMoveStraightRequest{
			Name:           "base1",
			MillisPerSec:   2.3,
			DistanceMillis: 1,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 1, 2.3, false})
		test.That(t, resp.Success, test.ShouldBeTrue)
		test.That(t, resp.DistanceMillis, test.ShouldEqual, 1)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return 2.2, err1
		}
		spinResp, err := server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:     "base1",
			AngleDeg: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 64.0, false})
		test.That(t, spinResp.Success, test.ShouldBeFalse)
		test.That(t, spinResp.Error, test.ShouldEqual, err1.Error())
		test.That(t, spinResp.AngleDeg, test.ShouldEqual, 2.2)

		injectBase.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			capArgs = []interface{}{ctx, angleDeg, degsPerSec, block}
			return angleDeg, nil
		}
		spinResp, err = server.BaseSpin(ctx, &pb.BaseSpinRequest{
			Name:       "base1",
			DegsPerSec: 20.3,
			AngleDeg:   4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{ctx, 4.5, 20.3, false})
		test.That(t, spinResp.Success, test.ShouldBeTrue)
		test.That(t, spinResp.AngleDeg, test.ShouldEqual, 4.5)
	})

	t.Run("ArmCurrentPosition", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			capName = name
			return nil
		}

		_, err := server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		pos := &pb.ArmPosition{X: 1, Y: 2, Z: 3, RX: 4, RY: 5, RZ: 6}
		injectArm.CurrentPositionFunc = func(ctx context.Context) (*pb.ArmPosition, error) {
			return nil, err1
		}

		_, err = server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectArm.CurrentPositionFunc = func(ctx context.Context) (*pb.ArmPosition, error) {
			return pos, nil
		}
		resp, err := server.ArmCurrentPosition(context.Background(), &pb.ArmCurrentPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Position, test.ShouldResemble, pos)
	})

	t.Run("ArmCurrentJointPositions", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			capName = name
			return nil
		}

		_, err := server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		pos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
		injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
			return nil, err1
		}

		_, err = server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectArm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
			return pos, nil
		}
		resp, err := server.ArmCurrentJointPositions(context.Background(), &pb.ArmCurrentJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Positions, test.ShouldResemble, pos)
	})

	t.Run("ArmMoveToPosition", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			capName = name
			return nil
		}

		_, err := server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		var capAP *pb.ArmPosition
		injectArm.MoveToPositionFunc = func(ctx context.Context, ap *pb.ArmPosition) error {
			capAP = ap
			return err1
		}

		pos := &pb.ArmPosition{X: 1, Y: 2, Z: 3, RX: 4, RY: 5, RZ: 6}
		_, err = server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAP, test.ShouldEqual, pos)

		injectArm.MoveToPositionFunc = func(ctx context.Context, ap *pb.ArmPosition) error {
			return nil
		}
		_, err = server.ArmMoveToPosition(context.Background(), &pb.ArmMoveToPositionRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ArmMoveToJointPositions", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			capName = name
			return nil
		}

		_, err := server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no arm")
		test.That(t, capName, test.ShouldEqual, "arm1")

		injectArm := &inject.Arm{}
		injectRobot.ArmByNameFunc = func(name string) arm.Arm {
			return injectArm
		}

		err1 := errors.New("whoops")
		var capJP *pb.JointPositions
		injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions) error {
			capJP = jp
			return err1
		}

		pos := &pb.JointPositions{Degrees: []float64{1.2, 3.4}}
		_, err = server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capJP, test.ShouldEqual, pos)

		injectArm.MoveToJointPositionsFunc = func(ctx context.Context, jp *pb.JointPositions) error {
			return nil
		}
		_, err = server.ArmMoveToJointPositions(context.Background(), &pb.ArmMoveToJointPositionsRequest{
			Name: "arm1",
			To:   pos,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Gripper", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.GripperByNameFunc = func(name string) gripper.Gripper {
			capName = name
			return nil
		}

		_, err := server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
		test.That(t, capName, test.ShouldEqual, "gripper1")

		_, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no gripper")
		test.That(t, capName, test.ShouldEqual, "gripper1")

		injectGripper := &inject.Gripper{}
		injectRobot.GripperByNameFunc = func(name string) gripper.Gripper {
			return injectGripper
		}

		err1 := errors.New("whoops")
		injectGripper.OpenFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.OpenFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.GripperOpen(context.Background(), &pb.GripperOpenRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)

		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return false, err1
		}
		_, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return false, nil
		}

		resp, err := server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeFalse)

		injectGripper.GrabFunc = func(ctx context.Context) (bool, error) {
			return true, nil
		}
		resp, err = server.GripperGrab(context.Background(), &pb.GripperGrabRequest{
			Name: "gripper1",
		})
		test.That(t, err, test.ShouldEqual, nil)
		test.That(t, resp.Grabbed, test.ShouldBeTrue)
	})

	t.Run("BoardStatus", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			capName = name
			return nil
		}

		_, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		injectBoard := &inject.Board{}
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			return injectBoard
		}

		err1 := errors.New("whoops")
		status := &pb.BoardStatus{
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
		}
		injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
			return nil, err1
		}
		_, err = server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		injectBoard.StatusFunc = func(ctx context.Context) (*pb.BoardStatus, error) {
			return status, nil
		}
		resp, err := server.BoardStatus(context.Background(), &pb.BoardStatusRequest{
			Name: "board1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Status, test.ShouldResemble, status)
	})

	t.Run("BoardMotor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			capName = name
			return nil
		}

		_, err := server.BoardMotorGo(context.Background(), &pb.BoardMotorGoRequest{
			BoardName: "board1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no board")
		test.That(t, capName, test.ShouldEqual, "board1")

		_, err = server.BoardMotorGoFor(context.Background(), &pb.BoardMotorGoForRequest{
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

		_, err = server.BoardMotorGo(context.Background(), &pb.BoardMotorGoRequest{
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
		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			capArgs = []interface{}{d, powerPct}
			return err1
		}
		_, err = server.BoardMotorGo(context.Background(), &pb.BoardMotorGoRequest{
			BoardName: "board1",
			MotorName: "motor1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, float32(0)})

		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			capArgs = []interface{}{d, powerPct}
			return nil
		}
		_, err = server.BoardMotorGo(context.Background(), &pb.BoardMotorGoRequest{
			BoardName: "board1",
			MotorName: "motor1",
			Direction: pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD,
			PowerPct:  2,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(2)})

		injectMotor.GoFunc = func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
			return errors.New("no")
		}
		injectMotor.GoForFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
			capArgs = []interface{}{d, rpm, revolutions}
			return err1
		}
		_, err = server.BoardMotorGoFor(context.Background(), &pb.BoardMotorGoForRequest{
			BoardName:   "board1",
			MotorName:   "motor1",
			Direction:   pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Rpm:         2.3,
			Revolutions: 4.5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})

		injectMotor.GoForFunc = func(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
			capArgs = []interface{}{d, rpm, revolutions}
			return nil
		}
		_, err = server.BoardMotorGoFor(context.Background(), &pb.BoardMotorGoForRequest{
			BoardName:   "board1",
			MotorName:   "motor1",
			Direction:   pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD,
			Rpm:         2.3,
			Revolutions: 4.5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capArgs, test.ShouldResemble, []interface{}{pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 2.3, 4.5})
	})

	t.Run("BoardServoMove", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.BoardByNameFunc = func(name string) board.Board {
			capName = name
			return nil
		}

		_, err := server.BoardServoMove(context.Background(), &pb.BoardServoMoveRequest{
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

		_, err = server.BoardServoMove(context.Background(), &pb.BoardServoMoveRequest{
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
		injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
			capAngle = angle
			return err1
		}
		_, err = server.BoardServoMove(context.Background(), &pb.BoardServoMoveRequest{
			BoardName: "board1",
			ServoName: "servo1",
			AngleDeg:  5,
		})
		test.That(t, err, test.ShouldEqual, err1)
		test.That(t, capAngle, test.ShouldEqual, 5)

		injectServo.MoveFunc = func(ctx context.Context, angle uint8) error {
			capAngle = angle
			return nil
		}
		_, err = server.BoardServoMove(context.Background(), &pb.BoardServoMoveRequest{
			BoardName: "board1",
			ServoName: "servo1",
			AngleDeg:  5,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, capAngle, test.ShouldEqual, 5)
	})

	t.Run("CameraFrame", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.CameraByNameFunc = func(name string) gostream.ImageSource {
			capName = name
			return nil
		}

		_, err := server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")
		test.That(t, capName, test.ShouldEqual, "camera1")

		injectImageSource := &inject.ImageSource{}
		injectRobot.CameraByNameFunc = func(name string) gostream.ImageSource {
			return injectImageSource
		}
		err1 := errors.New("whoops")
		injectImageSource.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return nil, nil, err1
		}
		_, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		var imgBuf bytes.Buffer
		test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

		var released bool
		injectImageSource.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return img, func() { released = true }, nil
		}

		resp, err := server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		released = false
		resp, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name:     "camera1",
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.MimeType, test.ShouldEqual, "image/png")
		test.That(t, resp.Frame, test.ShouldResemble, imgBuf.Bytes())

		released = false
		_, err = server.CameraFrame(context.Background(), &pb.CameraFrameRequest{
			Name:     "camera1",
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, released, test.ShouldBeTrue)
	})

	t.Run("CameraRenderFrame", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.CameraByNameFunc = func(name string) gostream.ImageSource {
			capName = name
			return nil
		}

		_, err := server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no camera")
		test.That(t, capName, test.ShouldEqual, "camera1")

		injectImageSource := &inject.ImageSource{}
		injectRobot.CameraByNameFunc = func(name string) gostream.ImageSource {
			return injectImageSource
		}
		err1 := errors.New("whoops")
		injectImageSource.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return nil, nil, err1
		}
		_, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldEqual, err1)

		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		var imgBuf bytes.Buffer
		test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)

		var released bool
		injectImageSource.NextFunc = func(ctx context.Context) (image.Image, func(), error) {
			return img, func() { released = true }, nil
		}

		resp, err := server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name: "camera1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		released = false
		resp, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name:     "camera1",
			MimeType: "image/png",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, released, test.ShouldBeTrue)
		test.That(t, resp.ContentType, test.ShouldEqual, "image/png")
		test.That(t, resp.Data, test.ShouldResemble, imgBuf.Bytes())

		released = false
		_, err = server.CameraRenderFrame(context.Background(), &pb.CameraRenderFrameRequest{
			Name:     "camera1",
			MimeType: "image/who",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
		test.That(t, released, test.ShouldBeTrue)
	})

	t.Run("Lidar", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.LidarByNameFunc = func(name string) lidar.Lidar {
			capName = name
			return nil
		}

		_, err := server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no lidar")
		test.That(t, capName, test.ShouldEqual, "lidar1")

		err1 := errors.New("whoops")

		device := &inject.Lidar{}
		injectRobot.LidarByNameFunc = func(name string) lidar.Lidar {
			return device
		}

		device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return nil, err1
		}
		_, err = server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.InfoFunc = func(ctx context.Context) (map[string]interface{}, error) {
			return map[string]interface{}{"hello": true}, nil
		}
		infoResp, err := server.LidarInfo(context.Background(), &pb.LidarInfoRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, infoResp.GetInfo().AsMap(), test.ShouldResemble, map[string]interface{}{"hello": true})

		device.StartFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.LidarStart(context.Background(), &pb.LidarStartRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StartFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.LidarStart(context.Background(), &pb.LidarStartRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.StopFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.LidarStop(context.Background(), &pb.LidarStopRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StopFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.LidarStop(context.Background(), &pb.LidarStopRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			return nil, err1
		}
		_, err = server.LidarScan(context.Background(), &pb.LidarScanRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		var capOptions lidar.ScanOptions
		ms := lidar.Measurements{lidar.NewMeasurement(0, 1), lidar.NewMeasurement(1, 2)}
		device.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
			capOptions = options
			return ms, nil
		}
		scanResp, err := server.LidarScan(context.Background(), &pb.LidarScanRequest{
			Name:  "lidar1",
			Count: 4, NoFilter: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, client.MeasurementsFromProto(scanResp.GetMeasurements()), test.ShouldResemble, ms)
		test.That(t, capOptions, test.ShouldResemble, lidar.ScanOptions{Count: 4, NoFilter: true})

		device.RangeFunc = func(ctx context.Context) (float64, error) {
			return 0, err1
		}
		_, err = server.LidarRange(context.Background(), &pb.LidarRangeRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.RangeFunc = func(ctx context.Context) (float64, error) {
			return 5, nil
		}
		rangeResp, err := server.LidarRange(context.Background(), &pb.LidarRangeRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rangeResp.GetRange(), test.ShouldEqual, 5)

		device.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{}, err1
		}
		_, err = server.LidarBounds(context.Background(), &pb.LidarBoundsRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.BoundsFunc = func(ctx context.Context) (r2.Point, error) {
			return r2.Point{4, 5}, nil
		}
		boundsResp, err := server.LidarBounds(context.Background(), &pb.LidarBoundsRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, boundsResp.GetX(), test.ShouldEqual, 4)
		test.That(t, boundsResp.GetY(), test.ShouldEqual, 5)

		device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.LidarAngularResolution(context.Background(), &pb.LidarAngularResolutionRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
			return 6.2, nil
		}
		angResp, err := server.LidarAngularResolution(context.Background(), &pb.LidarAngularResolutionRequest{
			Name: "lidar1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, angResp.GetAngularResolution(), test.ShouldEqual, 6.2)
	})

	t.Run("Sensor", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
			capName = name
			return nil
		}

		_, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldEqual, "compass1")

		err1 := errors.New("whoops")

		device := &inject.Compass{}
		injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
			return device
		}

		device.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return nil, err1
		}
		_, err = server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.ReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return []interface{}{1.2, 2.3}, nil
		}
		resp, err := server.SensorReadings(context.Background(), &pb.SensorReadingsRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		readings := make([]interface{}, 0, len(resp.Readings))
		for _, r := range resp.Readings {
			readings = append(readings, r.AsInterface())
		}
		test.That(t, readings, test.ShouldResemble, []interface{}{1.2, 2.3})
	})

	t.Run("Compass", func(t *testing.T) {
		server, injectRobot := newServer()
		var capName string
		injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
			capName = name
			return nil
		}

		_, err := server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no sensor")
		test.That(t, capName, test.ShouldEqual, "compass1")

		err1 := errors.New("whoops")

		device := &inject.Compass{}
		injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
			return device
		}

		device.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 1.2, nil
		}
		resp, err := server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Heading, test.ShouldResemble, 1.2)

		device.StartCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StartCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		device.StopCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		device.StopCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not relative")

		relDevice := &inject.RelativeCompass{}
		injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
			return relDevice
		}

		relDevice.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		_, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 1.2, nil
		}
		resp, err = server.CompassHeading(context.Background(), &pb.CompassHeadingRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Heading, test.ShouldResemble, 1.2)

		relDevice.StartCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.StartCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStartCalibration(context.Background(), &pb.CompassStartCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		relDevice.StopCalibrationFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.StopCalibrationFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassStopCalibration(context.Background(), &pb.CompassStopCalibrationRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)

		relDevice.MarkFunc = func(ctx context.Context) error {
			return err1
		}
		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldEqual, err1)
		relDevice.MarkFunc = func(ctx context.Context) error {
			return nil
		}
		_, err = server.CompassMark(context.Background(), &pb.CompassMarkRequest{
			Name: "compass1",
		})
		test.That(t, err, test.ShouldBeNil)
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
