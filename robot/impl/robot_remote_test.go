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
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func setupInjectRobotWithSuffx(logger golog.Logger, suffix string) *inject.Robot {
	injectRobot := &inject.Robot{}
	armNames := []resource.Name{
		arm.Named(fmt.Sprintf("arm1%s", suffix)),
		arm.Named(fmt.Sprintf("arm2%s", suffix)),
	}
	baseNames := []resource.Name{
		base.Named(fmt.Sprintf("base1%s", suffix)),
		base.Named(fmt.Sprintf("base2%s", suffix)),
	}
	boardNames := []resource.Name{
		board.Named(fmt.Sprintf("board1%s", suffix)),
		board.Named(fmt.Sprintf("board2%s", suffix)),
	}
	cameraNames := []resource.Name{
		camera.Named(fmt.Sprintf("camera1%s", suffix)),
		camera.Named(fmt.Sprintf("camera2%s", suffix)),
	}
	gripperNames := []resource.Name{
		gripper.Named(fmt.Sprintf("gripper1%s", suffix)),
		gripper.Named(fmt.Sprintf("gripper2%s", suffix)),
	}
	inputNames := []resource.Name{
		input.Named(fmt.Sprintf("inputController1%s", suffix)),
		input.Named(fmt.Sprintf("inputController2%s", suffix)),
	}
	motorNames := []resource.Name{
		motor.Named(fmt.Sprintf("motor1%s", suffix)),
		motor.Named(fmt.Sprintf("motor2%s", suffix)),
	}
	servoNames := []resource.Name{
		servo.Named(fmt.Sprintf("servo1%s", suffix)),
		servo.Named(fmt.Sprintf("servo2%s", suffix)),
	}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{fmt.Sprintf("remote1%s", suffix), fmt.Sprintf("remote2%s", suffix)}
	}

	injectRobot.ResourceNamesFunc = func() []resource.Name {
		return rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
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

	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		for _, rName := range injectRobot.ResourceNames() {
			if rName == name {
				switch name.Subtype {
				case arm.Subtype:
					return &fakearm.Arm{Name: name.Name}, nil
				case base.Subtype:
					return &fakebase.Base{Name: name.Name}, nil
				case board.Subtype:
					fakeBoard, err := fakeboard.NewBoard(context.Background(), config.Component{
						Name: name.Name,
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
					return fakeBoard, nil
				case camera.Subtype:
					return &fakecamera.Camera{Name: name.Name}, nil
				case gripper.Subtype:
					return &fakegripper.Gripper{Name: name.Name}, nil
				case input.Subtype:
					return &fakeinput.InputController{Name: name.Name}, nil
				case motor.Subtype:
					return &fakemotor.Motor{Name: name.Name}, nil
				case servo.Subtype:
					return &fakeservo.Servo{Name: name.Name}, nil
				}
				if rName.ResourceType == resource.ResourceTypeService {
					return struct{}{}, nil
				}
			}
		}
		return nil, rutils.NewResourceNotFoundError(name)
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
	robot := newRemoteRobot(
		context.Background(),
		wrapped,
		config.Remote{
			Name:   "one",
			Prefix: true,
		},
	)

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

	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	prefixedBaseNames := []resource.Name{base.Named("one.base1"), base.Named("one.base2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(base.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(base.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedBaseNames...)...),
	)

	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	prefixedBoardNames := []resource.Name{board.Named("one.board1"), board.Named("one.board2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(board.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(board.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedBoardNames...)...),
	)

	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	prefixedCameraNames := []resource.Name{camera.Named("one.camera1"), camera.Named("one.camera2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(camera.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(camera.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedCameraNames...)...),
	)

	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	prefixedGripperNames := []resource.Name{gripper.Named("one.gripper1"), gripper.Named("one.gripper2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(gripper.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(gripper.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedGripperNames...)...),
	)

	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	prefixedInputNames := []resource.Name{input.Named("one.inputController1"), input.Named("one.inputController2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(input.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(input.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedInputNames...)...),
	)

	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	prefixedMotorNames := []resource.Name{motor.Named("one.motor1"), motor.Named("one.motor2")}
	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(motor.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(motor.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedMotorNames...)...),
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
		utils.NewStringSet(servo.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(servo.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(prefixedServoNames...)...),
	)

	robot.conf.Prefix = false
	test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
		rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...))
	robot.conf.Prefix = true
	test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
		rdktestutils.ConcatResourceNames(
			prefixedArmNames,
			prefixedBaseNames,
			prefixedBoardNames,
			prefixedCameraNames,
			prefixedGripperNames,
			prefixedInputNames,
			prefixedMotorNames,
			prefixedServoNames,
		)...))

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return nil, errors.New("whoops")
	}

	robot.conf.Prefix = false
	_, err := arm.FromRobot(robot, "arm1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = arm.FromRobot(robot, "one.arm1")
	test.That(t, err, test.ShouldBeNil)
	_, err = arm.FromRobot(robot, "arm1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = base.FromRobot(robot, "base1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = base.FromRobot(robot, "one.base1")
	test.That(t, err, test.ShouldBeNil)
	_, err = base.FromRobot(robot, "base1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = board.FromRobot(robot, "board1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = board.FromRobot(robot, "one.board1")
	test.That(t, err, test.ShouldBeNil)
	_, err = board.FromRobot(robot, "board1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = camera.FromRobot(robot, "camera1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = camera.FromRobot(robot, "one.camera1")
	test.That(t, err, test.ShouldBeNil)
	_, err = camera.FromRobot(robot, "camera1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = gripper.FromRobot(robot, "gripper1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = gripper.FromRobot(robot, "one.gripper1")
	test.That(t, err, test.ShouldBeNil)
	_, err = gripper.FromRobot(robot, "gripper1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = sensor.FromRobot(robot, "sensor1")
	test.That(t, err, test.ShouldNotBeNil)
	robot.conf.Prefix = true
	_, err = sensor.FromRobot(robot, "one.sensor1")
	test.That(t, err, test.ShouldNotBeNil)
	_, err = sensor.FromRobot(robot, "sensor1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = servo.FromRobot(robot, "servo1")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = servo.FromRobot(robot, "one.servo1")
	test.That(t, err, test.ShouldBeNil)
	_, err = servo.FromRobot(robot, "servo1_what")
	test.That(t, err, test.ShouldNotBeNil)

	robot.conf.Prefix = false
	_, err = robot.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = robot.ResourceByName(arm.Named("one.arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = robot.ResourceByName(arm.Named("arm1_what"))
	test.That(t, err, test.ShouldBeError)

	wrapped.errRefresh = true
	err = robot.Refresh(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, "error refreshing")

	wrapped.errRefresh = false
	err = robot.Refresh(context.Background())
	test.That(t, err, test.ShouldBeNil)

	robot.conf.Prefix = false
	test.That(
		t,
		utils.NewStringSet(gripper.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet("pieceGripper", "pieceGripper2"),
	)
	robot.conf.Prefix = true
	test.That(
		t,
		utils.NewStringSet(gripper.NamesFromRobot(robot)...),
		test.ShouldResemble,
		utils.NewStringSet("one.pieceGripper", "one.pieceGripper2"),
	)

	robot.conf.Prefix = false
	_, err = gripper.FromRobot(robot, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	robot.conf.Prefix = true
	_, err = gripper.FromRobot(robot, "one.pieceGripper")
	test.That(t, err, test.ShouldBeNil)

	_, err = sensor.FromRobot(robot, "sensor1")
	test.That(t, err, test.ShouldNotBeNil)
	_, err = sensor.FromRobot(robot, "one.sensor1")
	test.That(t, err, test.ShouldNotBeNil)

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
	conf, err := config.FromReader(ctx, "somepath", strings.NewReader(confRaw), w.logger)
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
