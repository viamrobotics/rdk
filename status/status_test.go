package status_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/component/arm"
	fakearm "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/base"
	fakebase "go.viam.com/rdk/component/base/fake"
	"go.viam.com/rdk/component/board"
	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/camera"
	fakecamera "go.viam.com/rdk/component/camera/fake"
	"go.viam.com/rdk/component/gripper"
	fakegripper "go.viam.com/rdk/component/gripper/fake"
	"go.viam.com/rdk/component/input"
	fakeinput "go.viam.com/rdk/component/input/fake"
	"go.viam.com/rdk/component/motor"
	fakemotor "go.viam.com/rdk/component/motor/fake"
	"go.viam.com/rdk/component/servo"
	fakeservo "go.viam.com/rdk/component/servo/fake"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/status"
	"go.viam.com/rdk/testutils/inject"
)

func setupInjectRobotHelper(logger golog.Logger, withRemotes, refreshFail, isRemote bool) *inject.Robot {
	injectRobot := &inject.Robot{}

	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{
			arm.Named("arm1"),
			arm.Named("arm2"),
			resource.NameFromSubtype(framesystem.Subtype, ""),
		}
	}
	injectRobot.ArmNamesFunc = func() []string {
		return []string{"arm1", "arm2"}
	}
	injectRobot.GripperNamesFunc = func() []string {
		return []string{"gripper1", "gripper2"}
	}
	injectRobot.CameraNamesFunc = func() []string {
		return []string{"camera1", "camera2"}
	}
	injectRobot.BaseNamesFunc = func() []string {
		return []string{"base1", "base2"}
	}
	injectRobot.BoardNamesFunc = func() []string {
		return []string{"board1", "board2"}
	}
	injectRobot.SensorNamesFunc = func() []string {
		return []string{"sensor1", "sensor2"}
	}
	injectRobot.ServoNamesFunc = func() []string {
		return []string{"servo1", "servo2"}
	}
	injectRobot.MotorNamesFunc = func() []string {
		return []string{"motor1", "motor2"}
	}
	injectRobot.InputControllerNamesFunc = func() []string {
		return []string{"inputController1", "inputController2"}
	}
	injectRobot.FunctionNamesFunc = func() []string {
		return []string{"func1", "func2"}
	}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return &fakearm.Arm{Name: name.Name}, true
	}
	injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		return &fakearm.Arm{Name: name}, true
	}
	injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		return &fakebase.Base{Name: name}, true
	}
	injectRobot.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
		return &fakegripper.Gripper{Name: name}, true
	}
	injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
		return &fakecamera.Camera{Name: name}, true
	}
	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &fakeboard.Board{Name: name}, true
	}
	injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return nil, false
	}
	injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
		return &fakeservo.Servo{Name: name}, true
	}
	injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return &fakemotor.Motor{Name: name}, true
	}
	injectRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		return &fakeinput.InputController{Name: name}, true
	}

	if withRemotes {
		injectRobot.RemoteNamesFunc = func() []string {
			return []string{"remote1", "remote2"}
		}
		remote1 := setupInjectRobotHelper(logger, false, false, true)
		remote2 := setupInjectRobotHelper(logger, false, refreshFail, true)
		injectRobot.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
			switch name {
			case "remote1":
				return remote1, true
			case "remote2":
				return remote2, true
			}
			return nil, false
		}
	} else {
		injectRobot.RemoteNamesFunc = func() []string {
			return nil
		}
	}

	injectRobot.RefreshFunc = func(ctx context.Context) error {
		if isRemote && refreshFail {
			return errors.New("whoops")
		}
		return nil
	}

	return injectRobot
}

func setupInjectRobot(logger golog.Logger, withRemotes, refreshFail bool) *inject.Robot {
	return setupInjectRobotHelper(logger, withRemotes, refreshFail, false)
}

func TestCreateStatus(t *testing.T) {
	t.Run("with no remotes", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		robot := setupInjectRobot(logger, false, false)

		status, err := status.Create(context.Background(), robot)
		test.That(t, err, test.ShouldBeNil)
		//nolint:dupl
		test.That(t, status, test.ShouldResemble, &pb.Status{
			Arms: map[string]*pb.ArmStatus{
				"arm1": {},
				"arm2": {},
			},
			Bases: map[string]bool{
				"base1": true,
				"base2": true,
			},
			Grippers: map[string]bool{
				"gripper1": true,
				"gripper2": true,
			},
			Boards: map[string]*commonpb.BoardStatus{
				"board1": {},
				"board2": {},
			},
			Cameras: map[string]bool{
				"camera1": true,
				"camera2": true,
			},
			Sensors: map[string]*pb.SensorStatus{},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
				"servo2": {},
			},
			Motors: map[string]*pb.MotorStatus{
				"motor1": {},
				"motor2": {},
			},
			InputControllers: map[string]*pb.InputControllerStatus{
				"inputController1": {Events: []*pb.InputControllerEvent{
					{Time: &timestamppb.Timestamp{Seconds: -62135596800}, Event: "PositionChangeAbs", Control: "AbsoluteX", Value: 0.7},
				}},
				"inputController2": {Events: []*pb.InputControllerEvent{
					{Time: &timestamppb.Timestamp{Seconds: -62135596800}, Event: "PositionChangeAbs", Control: "AbsoluteX", Value: 0.7},
				}},
			},
			Functions: map[string]bool{
				"func1": true,
				"func2": true,
			},
			Services: map[string]bool{
				"rdk:service:frame_system": true,
			},
		})
	})

	t.Run("with remotes", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		robot := setupInjectRobot(logger, true, true)

		_, err := status.Create(context.Background(), robot)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

		logger = golog.NewTestLogger(t)
		robot = setupInjectRobot(logger, true, false)

		status, err := status.Create(context.Background(), robot)
		test.That(t, err, test.ShouldBeNil)
		// Status is the same as with no remotes because it's up to the
		// robot to utilize information from the remotes. We know
		// Refresh is called due to the failure above.
		//nolint:dupl
		test.That(t, status, test.ShouldResemble, &pb.Status{
			Arms: map[string]*pb.ArmStatus{
				"arm1": {},
				"arm2": {},
			},
			Bases: map[string]bool{
				"base1": true,
				"base2": true,
			},
			Grippers: map[string]bool{
				"gripper1": true,
				"gripper2": true,
			},
			Boards: map[string]*commonpb.BoardStatus{
				"board1": {},
				"board2": {},
			},
			Cameras: map[string]bool{
				"camera1": true,
				"camera2": true,
			},
			Sensors: map[string]*pb.SensorStatus{},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
				"servo2": {},
			},
			Motors: map[string]*pb.MotorStatus{
				"motor1": {},
				"motor2": {},
			},
			InputControllers: map[string]*pb.InputControllerStatus{
				"inputController1": {Events: []*pb.InputControllerEvent{
					{Time: &timestamppb.Timestamp{Seconds: -62135596800}, Event: "PositionChangeAbs", Control: "AbsoluteX", Value: 0.7},
				}},
				"inputController2": {Events: []*pb.InputControllerEvent{
					{Time: &timestamppb.Timestamp{Seconds: -62135596800}, Event: "PositionChangeAbs", Control: "AbsoluteX", Value: 0.7},
				}},
			},
			Functions: map[string]bool{
				"func1": true,
				"func2": true,
			},
			Services: map[string]bool{
				"rdk:service:frame_system": true,
			},
		})
	})
}
