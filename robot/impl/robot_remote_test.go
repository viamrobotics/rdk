package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/input"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"
	"go.viam.com/core/testutils/inject"

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
		return []string{fmt.Sprintf("sensor1%s", suffix), fmt.Sprintf("sensor2%s", suffix), fmt.Sprintf("forcematrix%s", suffix)}
	}
	injectRobot.ServoNamesFunc = func() []string {
		return []string{fmt.Sprintf("servo1%s", suffix), fmt.Sprintf("servo2%s", suffix)}
	}
	injectRobot.MotorNamesFunc = func() []string {
		return []string{fmt.Sprintf("motor1%s", suffix), fmt.Sprintf("motor2%s", suffix)}
	}
	injectRobot.InputControllerNamesFunc = func() []string {
		return []string{fmt.Sprintf("inputController1%s", suffix), fmt.Sprintf("inputController2%s", suffix)}
	}
	injectRobot.FunctionNamesFunc = func() []string {
		return []string{fmt.Sprintf("func1%s", suffix), fmt.Sprintf("func2%s", suffix)}
	}
	injectRobot.ServiceNamesFunc = func() []string {
		return []string{fmt.Sprintf("service1%s", suffix), fmt.Sprintf("service2%s", suffix)}
	}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named(fmt.Sprintf("arm1%s", suffix)), arm.Named(fmt.Sprintf("arm2%s", suffix))}
	}
	injectRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	injectRobot.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		if _, ok := utils.NewStringSet(injectRobot.RemoteNames()...)[name]; !ok {
			return nil, false
		}
		return &remoteRobot{conf: config.Remote{Name: name}}, true
	}
	injectRobot.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		if _, ok := utils.NewStringSet(injectRobot.ArmNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Arm{Name: name}, true
	}
	injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		if _, ok := utils.NewStringSet(injectRobot.BaseNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Base{Name: name}, true
	}
	injectRobot.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
		if _, ok := utils.NewStringSet(injectRobot.GripperNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Gripper{Name: name}, true
	}
	injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
		if _, ok := utils.NewStringSet(injectRobot.CameraNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Camera{Name: name}, true
	}
	injectRobot.LidarByNameFunc = func(name string) (lidar.Lidar, bool) {
		if _, ok := utils.NewStringSet(injectRobot.LidarNames()...)[name]; !ok {
			return nil, false
		}
		return fake.NewLidar(config.Component{Name: name}), true
	}
	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		if _, ok := utils.NewStringSet(injectRobot.BoardNames()...)[name]; !ok {
			return nil, false
		}
		fakeBoard, err := fake.NewBoard(context.Background(), config.Component{
			Name: name,
			ConvertedAttributes: &board.Config{
				Analogs: []board.AnalogConfig{
					{Name: "analog1"},
					{Name: "analog2"},
				},
				DigitalInterrupts: []board.DigitalInterruptConfig{
					{Name: "digital1"},
					{Name: "digital2"},
				},
			},
		}, logger)
		if err != nil {
			panic(err)
		}
		return fakeBoard, true
	}
	injectRobot.SensorByNameFunc = func(name string) (sensor.Sensor, bool) {
		if _, ok := utils.NewStringSet(injectRobot.SensorNames()...)[name]; !ok {
			return nil, false
		}
		if strings.HasPrefix(name, "forcematrix") {
			return &fake.ForceMatrix{Name: name}, true
		}
		return &fake.Compass{Name: name}, true
	}
	injectRobot.ServoByNameFunc = func(name string) (servo.Servo, bool) {
		if _, ok := utils.NewStringSet(injectRobot.ServoNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Servo{Name: name}, true
	}
	injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		if _, ok := utils.NewStringSet(injectRobot.MotorNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.Motor{Name: name}, true
	}
	injectRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		if _, ok := utils.NewStringSet(injectRobot.InputControllerNames()...)[name]; !ok {
			return nil, false
		}
		return &fake.InputController{Name: name}, true
	}
	injectRobot.ServiceByNameFunc = func(name string) (interface{}, bool) {
		if _, ok := utils.NewStringSet(injectRobot.ServiceNames()...)[name]; !ok {
			return nil, false
		}
		return struct{}{}, true
	}
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				// TODO: some kind of mapping based on resource name may be needed
				return &fake.Arm{Name: name.Name}, true
			}
		}
		return nil, false
	}

	return injectRobot
}

func setupInjectRobot(logger golog.Logger) *inject.Robot {
	return setupInjectRobotWithSuffx(logger, "")
}

func newResourceNameSet(values ...resource.Name) map[resource.Name]struct{} {
	set := make(map[resource.Name]struct{}, len(values))
	for _, val := range values {
		set[val] = struct{}{}
	}
	return set
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
	test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldResemble, utils.NewStringSet("one.sensor1", "one.sensor2", "one.forcematrix"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo1", "servo2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.ServoNames()...), test.ShouldResemble, utils.NewStringSet("one.servo1", "one.servo2"))

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("one.func1", "one.func2"))

	robot.conf.Prefix = false
	test.That(t, newResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, newResourceNameSet([]resource.Name{arm.Named("arm1"), arm.Named("arm2")}...))
	robot.conf.Prefix = true
	test.That(t, newResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, newResourceNameSet([]resource.Name{arm.Named("one.arm1"), arm.Named("one.arm2")}...))

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return nil, errors.New("whoops")
	}
	_, err := robot.Config(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	confGen := func() *config.Config {
		return &config.Config{
			Services: []config.Service{
				{
					Name: "frame_system",
					Type: "frame_system",
				},
			},
			Components: []config.Component{
				{
					Name:  "foo",
					Type:  "gripper",
					Model: "fake",
					Frame: &config.Frame{
						Parent: "bar",
					},
				},
				{
					Name:  "bar",
					Type:  "arm",
					Model: "fake",
					Frame: &config.Frame{
						Parent: "world",
					},
				},
				{
					Name:  "som",
					Type:  "camera",
					Model: "fake",
				},
			},
		}
	}

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return confGen(), nil
	}
	robot.conf.Prefix = true
	conf, err := robot.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Components[0].Name, test.ShouldEqual, "one.foo")
	test.That(t, conf.Components[0].Type, test.ShouldEqual, "gripper")
	test.That(t, conf.Components[0].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[0].Frame.Parent, test.ShouldEqual, "one.bar")
	test.That(t, conf.Components[1].Name, test.ShouldEqual, "one.bar")
	test.That(t, conf.Components[1].Type, test.ShouldEqual, "arm")
	test.That(t, conf.Components[1].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[1].Frame.Parent, test.ShouldEqual, "one.world")
	test.That(t, conf.Components[2].Name, test.ShouldEqual, "one.som")
	test.That(t, conf.Components[2].Type, test.ShouldEqual, "camera")
	test.That(t, conf.Components[2].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[2].Frame, test.ShouldBeNil)

	robot.conf.Prefix = false
	conf, err = robot.Config(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conf.Components[0].Name, test.ShouldEqual, "foo")
	test.That(t, conf.Components[0].Type, test.ShouldEqual, "gripper")
	test.That(t, conf.Components[0].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[0].Frame.Parent, test.ShouldEqual, "bar")
	test.That(t, conf.Components[1].Name, test.ShouldEqual, "bar")
	test.That(t, conf.Components[1].Type, test.ShouldEqual, "arm")
	test.That(t, conf.Components[1].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[1].Frame.Parent, test.ShouldEqual, "world")
	test.That(t, conf.Components[2].Name, test.ShouldEqual, "som")
	test.That(t, conf.Components[2].Type, test.ShouldEqual, "camera")
	test.That(t, conf.Components[2].Model, test.ShouldEqual, "fake")
	test.That(t, conf.Components[2].Frame, test.ShouldBeNil)

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
			"sensor1":     {},
			"sensor2":     {},
			"forcematrix": {},
		},
		Servos: map[string]*pb.ServoStatus{
			"servo1": {},
			"servo2": {},
		},
		Functions: map[string]bool{
			"func1": true,
			"func2": true,
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
			"one.sensor1":     {},
			"one.sensor2":     {},
			"one.forcematrix": {},
		},
		Servos: map[string]*pb.ServoStatus{
			"one.servo1": {},
			"one.servo2": {},
		},
		Functions: map[string]bool{
			"one.func1": true,
			"one.func2": true,
		},
	})

	robot.conf.Prefix = false
	_, ok := robot.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.ArmByName("one.arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = robot.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	base1, ok := robot.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	robot.conf.Prefix = true
	base1, ok = robot.BaseByName("one.base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	_, ok = robot.BaseByName("base1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	gripper1, ok := robot.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	robot.conf.Prefix = true
	gripper1, ok = robot.GripperByName("one.gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	_, ok = robot.GripperByName("gripper1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	camera1, ok := robot.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	robot.conf.Prefix = true
	camera1, ok = robot.CameraByName("one.camera1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	_, ok = robot.CameraByName("camera1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	lidar1, ok := robot.LidarByName("lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	robot.conf.Prefix = true
	lidar1, ok = robot.LidarByName("one.lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	_, ok = robot.LidarByName("lidar1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	board1, ok := robot.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1")
	robot.conf.Prefix = true
	board1, ok = robot.BoardByName("one.board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1")
	_, ok = robot.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	sensor1, ok := robot.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	robot.conf.Prefix = true
	sensor1, ok = robot.SensorByName("one.sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	_, ok = robot.SensorByName("sensor1_what")
	test.That(t, ok, test.ShouldBeFalse)
	fsm, ok := robot.SensorByName("forcematrix")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix")

	robot.conf.Prefix = false
	servo1, ok := robot.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, servo1.(*proxyServo).actual.(*fake.Servo).Name, test.ShouldEqual, "servo1")
	robot.conf.Prefix = true
	servo1, ok = robot.ServoByName("one.servo1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, servo1.(*proxyServo).actual.(*fake.Servo).Name, test.ShouldEqual, "servo1")
	_, ok = robot.ServoByName("servo1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	_, ok = robot.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.ResourceByName(arm.Named("one.arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = robot.ResourceByName(arm.Named("arm1_what"))
	test.That(t, ok, test.ShouldBeFalse)

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
	pieceGripper, ok := robot.GripperByName("pieceGripper")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, pieceGripper.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "pieceGripper")
	robot.conf.Prefix = true
	pieceGripper, ok = robot.GripperByName("one.pieceGripper")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, pieceGripper.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "pieceGripper")

	_, ok = robot.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = robot.SensorByName("one.sensor1")
	test.That(t, ok, test.ShouldBeFalse)
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
