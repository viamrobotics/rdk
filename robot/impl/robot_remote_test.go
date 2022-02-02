package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

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
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/component/servo"
	fakeservo "go.viam.com/rdk/component/servo/fake"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
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
	baseNames := []resource.Name{
		base.Named(fmt.Sprintf("base1%s", suffix)),
		base.Named(fmt.Sprintf("base2%s", suffix)),
	}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{fmt.Sprintf("remote1%s", suffix), fmt.Sprintf("remote2%s", suffix)}
	}
	injectRobot.BoardNamesFunc = func() []string {
		return rdktestutils.ExtractNames(boardNames...)
	}
	injectRobot.BoardNamesFunc = func() []string {
		return rdktestutils.ExtractNames(boardNames...)
	}
	injectRobot.GripperNamesFunc = func() []string {
		return rdktestutils.ExtractNames(gripperNames...)
	}
	injectRobot.CameraNamesFunc = func() []string {
		return rdktestutils.ExtractNames(cameraNames...)
	}
	injectRobot.BaseNamesFunc = func() []string {
		return rdktestutils.ExtractNames(baseNames...)
	}
	injectRobot.ServoNamesFunc = func() []string {
		return rdktestutils.ExtractNames(servoNames...)
	}
	injectRobot.MotorNamesFunc = func() []string {
		return rdktestutils.ExtractNames(motorNames...)
	}
	injectRobot.InputControllerNamesFunc = func() []string {
		return rdktestutils.ExtractNames(inputNames...)
	}
	injectRobot.FunctionNamesFunc = func() []string {
		return []string{fmt.Sprintf("func1%s", suffix), fmt.Sprintf("func2%s", suffix)}
	}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
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

	injectRobot.BaseByNameFunc = func(name string) (base.Base, bool) {
		if _, ok := utils.NewStringSet(injectRobot.BaseNames()...)[name]; !ok {
			return nil, false
		}
		return &fakebase.Base{Name: name}, true
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
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.Subtype {
				case arm.Subtype:
					return &fakearm.Arm{Name: name.Name}, true
				case base.Subtype:
					return &fakebase.Base{Name: name.Name}, true
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
				if rName.ResourceType == resource.ResourceTypeService {
					return struct{}{}, true
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
		utils.NewStringSet(arm.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(arm.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedArmNames...)...),
	)

	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	prefixedGripperNames := []resource.Name{gripper.Named("one.gripper1"), gripper.Named("one.gripper2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedGripperNames...)...),
	)

	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	prefixedCameraNames := []resource.Name{camera.Named("one.camera1"), camera.Named("one.camera2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedCameraNames...)...),
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
		utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedBoardNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)

	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	prefixedServoNames := []resource.Name{servo.Named("one.servo1"), servo.Named("one.servo2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedServoNames...)...),
	)

	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	prefixedMotorNames := []resource.Name{motor.Named("one.motor1"), motor.Named("one.motor2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedMotorNames...)...),
	)

	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	prefixedBaseNames := []resource.Name{base.Named("one.base1"), base.Named("one.base2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedBaseNames...)...),
	)

	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	prefixedInputNames := []resource.Name{input.Named("one.inputController1"), input.Named("one.inputController2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(robot.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(robot.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedInputNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2"))
	robot.conf.Prefix = true
	test.That(t, utils.NewStringSet(robot.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("one.func1", "one.func2"))

	robot.conf.Prefix = false
	test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
		rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...))
	robot.conf.Prefix = true
	test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
		rdktestutils.ConcatResourceNames(
			prefixedArmNames,
			prefixedBoardNames,
			prefixedGripperNames,
			prefixedCameraNames,
			prefixedServoNames,
			prefixedMotorNames,
			prefixedInputNames,
			prefixedBaseNames,
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
		Boards: map[string]*commonpb.BoardStatus{
			"board1": {},
			"board2": {},
		},
		Cameras: map[string]bool{
			"camera1": true,
			"camera2": true,
		},
		Sensors: nil,
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
		Boards: map[string]*commonpb.BoardStatus{
			"one.board1": {},
			"one.board2": {},
		},
		Cameras: map[string]bool{
			"one.camera1": true,
			"one.camera2": true,
		},
		Sensors: nil,
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
	_, ok := arm.FromRobot(robot, "arm1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = arm.FromRobot(robot, "one.arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = arm.FromRobot(robot, "arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	robot.conf.Prefix = false
	_, ok = robot.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	robot.conf.Prefix = true
	_, ok = robot.BaseByName("one.base1")
	test.That(t, ok, test.ShouldBeTrue)
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
	_, ok = sensor.FromRobot(robot, "sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	robot.conf.Prefix = true
	_, ok = sensor.FromRobot(robot, "one.sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = sensor.FromRobot(robot, "sensor1_what")
	test.That(t, ok, test.ShouldBeFalse)

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

	_, ok = sensor.FromRobot(robot, "sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = sensor.FromRobot(robot, "one.sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	test.That(t, wrapped.Robot.Close(context.Background()), test.ShouldBeNil)
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
	conf, err := config.FromReader(ctx, "somepath", strings.NewReader(confRaw))
	if err != nil {
		return err
	}

	robot, err := New(ctx, conf, w.logger)
	if err != nil {
		return err
	}
	w.Robot = robot
	return nil
}
