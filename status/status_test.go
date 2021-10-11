package status_test

import (
	"context"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"
	"go.viam.com/core/status"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func setupInjectRobotHelper(logger golog.Logger, withRemotes, refreshFail, isRemote bool) *inject.Robot {
	injectRobot := &inject.Robot{}

	injectRobot.ArmNamesFunc = func() []string {
		return []string{"arm1", "arm2"}
	}
	injectRobot.GripperNamesFunc = func() []string {
		return []string{"gripper1", "gripper2"}
	}
	injectRobot.CameraNamesFunc = func() []string {
		return []string{"camera1", "camera2"}
	}
	injectRobot.LidarNamesFunc = func() []string {
		return []string{"lidar1", "lidar2"}
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
	injectRobot.FunctionNamesFunc = func() []string {
		return []string{"func1", "func2"}
	}
	injectRobot.ServiceNamesFunc = func() []string {
		return []string{"service1", "service2"}
	}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		return &fake.Arm{Name: name}, true
	}
	injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		return &fake.Base{Name: name}, true
	}
	injectRobot.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
		return &fake.Gripper{Name: name}, true
	}
	injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
		return &fake.Camera{Name: name}, true
	}
	injectRobot.LidarByNameFunc = func(name string) (lidar.Lidar, bool) {
		return &fake.Lidar{Name: name}, true
	}
	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return &fake.Board{Name: name}, true
	}
	injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		return &fake.Compass{Name: name}, true
	}
	injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
		return &fake.Servo{Name: name}, true
	}
	injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		return &fake.Motor{Name: name}, true
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
			Boards: map[string]*pb.BoardStatus{
				"board1": {},
				"board2": {},
			},
			Cameras: map[string]bool{
				"camera1": true,
				"camera2": true,
			},
			Lidars: map[string]bool{
				"lidar1": true,
				"lidar2": true,
			},
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: "compass",
				},
				"sensor2": {
					Type: "compass",
				},
			},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
				"servo2": {},
			},
			Motors: map[string]*pb.MotorStatus{
				"motor1": {},
				"motor2": {},
			},
			Functions: map[string]bool{
				"func1": true,
				"func2": true,
			},
			Services: map[string]bool{
				"service1": true,
				"service2": true,
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
			Boards: map[string]*pb.BoardStatus{
				"board1": {},
				"board2": {},
			},
			Cameras: map[string]bool{
				"camera1": true,
				"camera2": true,
			},
			Lidars: map[string]bool{
				"lidar1": true,
				"lidar2": true,
			},
			Sensors: map[string]*pb.SensorStatus{
				"sensor1": {
					Type: "compass",
				},
				"sensor2": {
					Type: "compass",
				},
			},
			Servos: map[string]*pb.ServoStatus{
				"servo1": {},
				"servo2": {},
			},
			Motors: map[string]*pb.MotorStatus{
				"motor1": {},
				"motor2": {},
			},
			Functions: map[string]bool{
				"func1": true,
				"func2": true,
			},
			Services: map[string]bool{
				"service1": true,
				"service2": true,
			},
		})
	})
}
