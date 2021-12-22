package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/component/board"
	fakeboard "go.viam.com/core/component/board/fake"
	"go.viam.com/core/component/camera"
	fakecamera "go.viam.com/core/component/camera/fake"
	"go.viam.com/core/component/gripper"
	fakegripper "go.viam.com/core/component/gripper/fake"
	"go.viam.com/core/component/input"
	fakeinput "go.viam.com/core/component/input/fake"
	"go.viam.com/core/component/motor"
	fakemotor "go.viam.com/core/component/motor/fake"
	"go.viam.com/core/component/servo"
	fakeservo "go.viam.com/core/component/servo/fake"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/sensor"
	coretestutils "go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func setupInjectRobotWithSuffx(logger golog.Logger, suffix string) *inject.Robot {
	injectRobot := &inject.Robot{}
	armNames := []resource.Name{
		arm.Named(fmt.Sprintf("arm1%s", suffix)),
		arm.Named(fmt.Sprintf("arm2%s", suffix)),
	}
	boardNames := []resource.Name{
		board.Named(fmt.Sprintf("board1%s", suffix)),
		board.Named(fmt.Sprintf("board2%s", suffix)),
	}
	gripperNames := []resource.Name{
		gripper.Named(fmt.Sprintf("gripper1%s", suffix)),
		gripper.Named(fmt.Sprintf("gripper2%s", suffix)),
	}
	cameraNames := []resource.Name{
		camera.Named(fmt.Sprintf("camera1%s", suffix)),
		camera.Named(fmt.Sprintf("camera2%s", suffix)),
	}
	servoNames := []resource.Name{
		servo.Named(fmt.Sprintf("servo1%s", suffix)),
		servo.Named(fmt.Sprintf("servo2%s", suffix)),
	}
	motorNames := []resource.Name{
		motor.Named(fmt.Sprintf("motor1%s", suffix)),
		motor.Named(fmt.Sprintf("motor2%s", suffix)),
	}
	inputNames := []resource.Name{
		input.Named(fmt.Sprintf("inputController1%s", suffix)),
		input.Named(fmt.Sprintf("inputController2%s", suffix)),
	}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{fmt.Sprintf("remote1%s", suffix), fmt.Sprintf("remote2%s", suffix)}
	}
	injectRobot.ArmNamesFunc = func() []string {
		return coretestutils.ExtractNames(armNames...)
	}
	injectRobot.BoardNamesFunc = func() []string {
		return coretestutils.ExtractNames(boardNames...)
	}
	injectRobot.GripperNamesFunc = func() []string {
		return coretestutils.ExtractNames(gripperNames...)
	}
	injectRobot.CameraNamesFunc = func() []string {
		return coretestutils.ExtractNames(cameraNames...)
	}
	injectRobot.BaseNamesFunc = func() []string {
		return []string{fmt.Sprintf("base1%s", suffix), fmt.Sprintf("base2%s", suffix)}
	}
	injectRobot.SensorNamesFunc = func() []string {
		return []string{fmt.Sprintf("sensor1%s", suffix), fmt.Sprintf("sensor2%s", suffix), fmt.Sprintf("forcematrix%s", suffix)}
	}
	injectRobot.ServoNamesFunc = func() []string {
		return coretestutils.ExtractNames(servoNames...)
	}
	injectRobot.MotorNamesFunc = func() []string {
		return coretestutils.ExtractNames(motorNames...)
	}
	injectRobot.InputControllerNamesFunc = func() []string {
		return coretestutils.ExtractNames(inputNames...)
	}
	injectRobot.FunctionNamesFunc = func() []string {
		return []string{fmt.Sprintf("func1%s", suffix), fmt.Sprintf("func2%s", suffix)}
	}
	injectRobot.ServiceNamesFunc = func() []string {
		return []string{fmt.Sprintf("service1%s", suffix), fmt.Sprintf("service2%s", suffix)}
	}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return coretestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
		)
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
	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		if _, ok := utils.NewStringSet(injectRobot.BoardNames()...)[name]; !ok {
			return nil, false
		}
		fakeBoard, err := fakeboard.NewBoard(context.Background(), config.Component{
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
		return &fakegripper.Gripper{Name: name}, true
	}
	injectRobot.CameraByNameFunc = func(name string) (camera.Camera, bool) {
		if _, ok := utils.NewStringSet(injectRobot.CameraNames()...)[name]; !ok {
			return nil, false
		}
		return &fakecamera.Camera{Name: name}, true
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
		return &fakeservo.Servo{Name: name}, true
	}
	injectRobot.MotorByNameFunc = func(name string) (motor.Motor, bool) {
		if _, ok := utils.NewStringSet(injectRobot.MotorNames()...)[name]; !ok {
			return nil, false
		}
		return &fakemotor.Motor{Name: name}, true
	}
	injectRobot.InputControllerByNameFunc = func(name string) (input.Controller, bool) {
		if _, ok := utils.NewStringSet(injectRobot.InputControllerNames()...)[name]; !ok {
			return nil, false
		}
		return &fakeinput.InputController{Name: name}, true
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
				switch name.Subtype {
				case arm.Subtype:
					return &fake.Arm{Name: name.Name}, true
				case board.Subtype:
					return injectRobot.BoardByNameFunc(name.Name)
				case servo.Subtype:
					return &fakeservo.Servo{Name: name.Name}, true
				case gripper.Subtype:
					return &fakegripper.Gripper{Name: name.Name}, true
				case camera.Subtype:
					return &fakecamera.Camera{Name: name.Name}, true
				case motor.Subtype:
					return &fakemotor.Motor{Name: name.Name}, true
				case input.Subtype:
					return &fakeinput.InputController{Name: name.Name}, true
				}
			}
		}
		return nil, false
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

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	prefixedArmNames := []resource.Name{arm.Named("one.arm1"), arm.Named("one.arm2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(armNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedArmNames...)...),
	)

	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	prefixedGripperNames := []resource.Name{gripper.Named("one.gripper1"), gripper.Named("one.gripper2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedGripperNames...)...),
	)

	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	prefixedCameraNames := []resource.Name{camera.Named("one.camera1"), camera.Named("one.camera2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedCameraNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("one.base1", "one.base2"))

	// Board

	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	prefixedBoardNames := []resource.Name{board.Named("one.board1"), board.Named("one.board2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(boardNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedBoardNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix"))
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet("one.sensor1", "one.sensor2", "one.forcematrix"),
	)

	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	prefixedServoNames := []resource.Name{servo.Named("one.servo1"), servo.Named("one.servo2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedServoNames...)...),
	)

	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	prefixedMotorNames := []resource.Name{motor.Named("one.motor1"), motor.Named("one.motor2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedMotorNames...)...),
	)

	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	prefixedInputNames := []resource.Name{input.Named("one.inputController1"), input.Named("one.inputController2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(inputNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(coretestutils.ExtractNames(prefixedInputNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("one.func1", "one.func2"))

	robot.conf.Prefix = false
	test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(
		coretestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
		)...))
	robot.conf.Prefix = true
	test.That(t, coretestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(
		coretestutils.ConcatResourceNames(
			prefixedArmNames,
			prefixedBoardNames,
			prefixedGripperNames,
			prefixedCameraNames,
			prefixedServoNames,
			prefixedMotorNames,
			prefixedInputNames,
		)...))

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
	_, ok = robot.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.GripperByName("one.gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = robot.GripperByName("gripper1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	_, ok = robot.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.CameraByName("one.camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = robot.CameraByName("camera1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false

	robot.conf.Prefix = false
	_, ok = robot.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.BoardByName("one.board1")
	test.That(t, ok, test.ShouldBeTrue)
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
	_, ok = robot.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.ServoByName("one.servo1")
	test.That(t, ok, test.ShouldBeTrue)
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
	test.That(
		t,
		utils.NewStringSet(robot.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet("one.pieceGripper", "one.pieceGripper2"),
	)

	robot.conf.Prefix = false
	_, ok = robot.GripperByName("pieceGripper")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.GripperByName("one.pieceGripper")
	test.That(t, ok, test.ShouldBeTrue)

	_, ok = robot.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = robot.SensorByName("one.sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, robot.Close(), test.ShouldBeNil)
	test.That(t, wrapped.Robot.Close(), test.ShouldBeNil)
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
