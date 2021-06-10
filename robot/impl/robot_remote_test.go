package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func setupInjectRobotWithSuffx(logger golog.Logger, suffix string) *inject.Robot {
	injectRobot := &inject.Robot{}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{fmt.Sprintf("remote1%s", suffix), fmt.Sprintf("remote2%s", suffix)}
	}
	injectRobot.ArmNamesFunc = func() []string {
		return []string{fmt.Sprintf("arm1%s", suffix), fmt.Sprintf("arm2%s", suffix)}
	}
	injectRobot.GripperNamesFunc = func() []string {
		return []string{fmt.Sprintf("gripper1%s", suffix), fmt.Sprintf("gripper2%s", suffix)}
	}
	injectRobot.CameraNamesFunc = func() []string {
		return []string{fmt.Sprintf("camera1%s", suffix), fmt.Sprintf("camera2%s", suffix)}
	}
	injectRobot.LidarNamesFunc = func() []string {
		return []string{fmt.Sprintf("lidar1%s", suffix), fmt.Sprintf("lidar2%s", suffix)}
	}
	injectRobot.BaseNamesFunc = func() []string {
		return []string{fmt.Sprintf("base1%s", suffix), fmt.Sprintf("base2%s", suffix)}
	}
	injectRobot.BoardNamesFunc = func() []string {
		return []string{fmt.Sprintf("board1%s", suffix), fmt.Sprintf("board2%s", suffix)}
	}
	injectRobot.SensorNamesFunc = func() []string {
		return []string{fmt.Sprintf("sensor1%s", suffix), fmt.Sprintf("sensor2%s", suffix)}
	}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.RemoteByNameFunc = func(name string) robot.Robot {
		if _, ok := utils.NewStringSet(injectRobot.RemoteNames()...)[name]; !ok {
			return nil
		}
		return &remoteRobot{conf: config.Remote{Name: name}}
	}
	injectRobot.ArmByNameFunc = func(name string) arm.Arm {
		if _, ok := utils.NewStringSet(injectRobot.ArmNames()...)[name]; !ok {
			return nil
		}
		return &fake.Arm{Name: name}
	}
	injectRobot.BaseByNameFunc = func(name string) base.Base {
		if _, ok := utils.NewStringSet(injectRobot.BaseNames()...)[name]; !ok {
			return nil
		}
		return &fake.Base{Name: name}
	}
	injectRobot.GripperByNameFunc = func(name string) gripper.Gripper {
		if _, ok := utils.NewStringSet(injectRobot.GripperNames()...)[name]; !ok {
			return nil
		}
		return &fake.Gripper{Name: name}
	}
	injectRobot.CameraByNameFunc = func(name string) camera.Camera {
		if _, ok := utils.NewStringSet(injectRobot.CameraNames()...)[name]; !ok {
			return nil
		}
		return &fake.Camera{Name: name}
	}
	injectRobot.LidarByNameFunc = func(name string) lidar.Lidar {
		if _, ok := utils.NewStringSet(injectRobot.LidarNames()...)[name]; !ok {
			return nil
		}
		return fake.NewLidar(name)
	}
	injectRobot.BoardByNameFunc = func(name string) board.Board {
		if _, ok := utils.NewStringSet(injectRobot.BoardNames()...)[name]; !ok {
			return nil
		}
		fakeBoard, err := board.NewFakeBoard(context.Background(), board.Config{
			Name: name,
			Motors: []board.MotorConfig{
				{Name: "motor1"},
				{Name: "motor2"},
			},
			Servos: []board.ServoConfig{
				{Name: "servo1"},
				{Name: "servo2"},
			},
			Analogs: []board.AnalogConfig{
				{Name: "analog1"},
				{Name: "analog2"},
			},
			DigitalInterrupts: []board.DigitalInterruptConfig{
				{Name: "digital1"},
				{Name: "digital2"},
			},
		}, logger)
		if err != nil {
			panic(err)
		}
		return fakeBoard
	}
	injectRobot.SensorByNameFunc = func(name string) sensor.Sensor {
		if _, ok := utils.NewStringSet(injectRobot.SensorNames()...)[name]; !ok {
			return nil
		}
		return &fake.Compass{Name: name}
	}

	return injectRobot
}

func setupInjectRobot(logger golog.Logger) *inject.Robot {
	return setupInjectRobotWithSuffx(logger, "")
}

func TestRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)

	injectRobot := setupInjectRobot(logger)

	wrapped := &dummyRemoteRobotWrapper{injectRobot, logger, false}
	robot := newRemoteRobot(wrapped, config.Remote{
		Name:   "one",
		Prefix: true,
	})

	robot.conf.Prefix = false
	test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
	robot.conf.Prefix = true
	test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("one.arm1", "one.arm2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldResemble, utils.NewStringSet("one.gripper1", "one.gripper2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldResemble, utils.NewStringSet("one.camera1", "one.camera2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldResemble, utils.NewStringSet("one.lidar1", "one.lidar2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("one.base1", "one.base2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("one.board1", "one.board2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldResemble, utils.NewStringSet("one.sensor1", "one.sensor2"))

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return nil, errors.New("whoops")
	}
	_, err := robot.Config(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	someConfig := &config.Config{
		Components: []config.Component{
			{
				Name:   "foo",
				Parent: "bar",
			},
			{
				Name:   "bar",
				Parent: "",
			},
		},
	}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return someConfig, nil
	}
	robot.conf.Prefix = true
	conf, err := robot.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Components[0].Name, test.ShouldEqual, "one.foo")
	test.That(t, conf.Components[0].Parent, test.ShouldEqual, "one.bar")
	test.That(t, conf.Components[1].Name, test.ShouldEqual, "one.bar")
	test.That(t, conf.Components[1].Parent, test.ShouldEqual, "")

	robot.conf.Prefix = false
	conf, err = robot.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Components[0].Name, test.ShouldEqual, "foo")
	test.That(t, conf.Components[0].Parent, test.ShouldEqual, "bar")
	test.That(t, conf.Components[1].Name, test.ShouldEqual, "bar")
	test.That(t, conf.Components[1].Parent, test.ShouldEqual, "")

	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return nil, errors.New("whoops")
	}
	_, err = robot.Status(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	someStatus := &pb.Status{
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
			"sensor1": {},
			"sensor2": {},
		},
	}
	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) {
		return someStatus, nil
	}
	robot.conf.Prefix = false
	status, err := robot.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, someStatus)
	robot.conf.Prefix = true
	status, err = robot.Status(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, &pb.Status{
		Arms: map[string]*pb.ArmStatus{
			"one.arm1": {},
			"one.arm2": {},
		},
		Bases: map[string]bool{
			"one.base1": true,
			"one.base2": true,
		},
		Grippers: map[string]bool{
			"one.gripper1": true,
			"one.gripper2": true,
		},
		Boards: map[string]*pb.BoardStatus{
			"one.board1": {},
			"one.board2": {},
		},
		Cameras: map[string]bool{
			"one.camera1": true,
			"one.camera2": true,
		},
		Lidars: map[string]bool{
			"one.lidar1": true,
			"one.lidar2": true,
		},
		Sensors: map[string]*pb.SensorStatus{
			"one.sensor1": {},
			"one.sensor2": {},
		},
	})

	robot.conf.Prefix = false
	arm1 := robot.ArmByName("arm1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	robot.conf.Prefix = true
	arm1 = robot.ArmByName("one.arm1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	test.That(t, robot.ArmByName("arm1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	base1 := robot.BaseByName("base1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	robot.conf.Prefix = true
	base1 = robot.BaseByName("one.base1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	test.That(t, robot.BaseByName("base1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	gripper1 := robot.GripperByName("gripper1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	robot.conf.Prefix = true
	gripper1 = robot.GripperByName("one.gripper1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	test.That(t, robot.GripperByName("gripper1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	camera1 := robot.CameraByName("camera1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	robot.conf.Prefix = true
	camera1 = robot.CameraByName("one.camera1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	test.That(t, robot.CameraByName("camera1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	lidar1 := robot.LidarByName("lidar1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	robot.conf.Prefix = true
	lidar1 = robot.LidarByName("one.lidar1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	test.That(t, robot.LidarByName("lidar1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	board1 := robot.BoardByName("board1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	robot.conf.Prefix = true
	board1 = robot.BoardByName("one.board1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	test.That(t, robot.BoardByName("board1_what"), test.ShouldBeNil)

	robot.conf.Prefix = false
	sensor1 := robot.SensorByName("sensor1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	robot.conf.Prefix = true
	sensor1 = robot.SensorByName("one.sensor1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	test.That(t, robot.SensorByName("sensor1_what"), test.ShouldBeNil)

	wrapped.errRefresh = true
	err = robot.Refresh(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "error refreshing")

	wrapped.errRefresh = false
	err = robot.Refresh(context.Background())
	test.That(t, err, test.ShouldBeNil)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldResemble, utils.NewStringSet("pieceGripper", "pieceGripper2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldResemble, utils.NewStringSet("one.pieceGripper", "one.pieceGripper2"))

	robot.conf.Prefix = false
	pieceGripper := robot.GripperByName("pieceGripper")
	test.That(t, pieceGripper.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "pieceGripper")
	robot.conf.Prefix = true
	pieceGripper = robot.GripperByName("one.pieceGripper")
	test.That(t, pieceGripper.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "pieceGripper")

	test.That(t, robot.SensorByName("sensor1"), test.ShouldBeNil)
	test.That(t, robot.SensorByName("one.sensor1"), test.ShouldBeNil)
}

type dummyRemoteRobotWrapper struct {
	robot.Robot
	logger     golog.Logger
	errRefresh bool
}

func (w *dummyRemoteRobotWrapper) Refresh(ctx context.Context) error {
	if w.errRefresh {
		return errors.New("error refreshing")
	}
	confRaw := `{
    "components": [
        {
            "model": "fake",
            "name": "pieceGripper",
            "type": "gripper"
        },
        {
            "model": "fake",
            "name": "pieceGripper2",
            "type": "gripper"
        }
    ]
}`
	conf, err := config.FromReader("somepath", strings.NewReader(confRaw))
	if err != nil {
		return err
	}

	robot, err := New(context.Background(), conf, w.logger)
	if err != nil {
		return err
	}
	w.Robot = robot
	return nil
}
