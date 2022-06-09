package robotimpl

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/board"
	fakeboard "go.viam.com/rdk/component/board/fake"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/component/input/fake"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	viz "go.viam.com/rdk/vision"
)

func TestManagerForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}

	test.That(t, manager.RemoteNames(), test.ShouldBeEmpty)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(manager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("base1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("board1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(camera.Named("camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("camera1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(gripper.Named("gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("gripper1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("motor1_what"))
	test.That(t, err, test.ShouldBeError)
	_, err = manager.ResourceByName(servo.Named("servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("servo_what"))
	test.That(t, err, test.ShouldBeError)
}

func TestManagerMergeNamesWithRemotes(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(manager.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(manager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("base1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("base1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(base.Named("base1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("board1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("board1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(board.Named("board1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(camera.Named("camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("camera1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("camera1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(camera.Named("camera1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(gripper.Named("gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("gripper1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("gripper1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(gripper.Named("gripper1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("motor1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("motor1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(motor.Named("motor1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = manager.ResourceByName(servo.Named("servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("servo1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("servo1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(servo.Named("servo1_what"))
	test.That(t, err, test.ShouldBeError)
}

func TestManagerResourceRemoteName(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := &inject.RemoteRobot{}
	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	injectRobot.ResourceNamesFunc = func() []resource.Name { return armNames }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) { return struct{}{}, nil }
	injectRobot.LoggerFunc = func() golog.Logger { return logger }

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()

	injectRemote := &inject.RemoteRobot{}
	injectRemote.ResourceNamesFunc = func() []resource.Name { return rdktestutils.AddSuffixes(armNames, "_r1") }
	injectRemote.ResourceByNameFunc = func(name resource.Name) (interface{}, error) { return struct{}{}, nil }
	injectRemote.LoggerFunc = func() golog.Logger { return logger }
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), injectRemote, config.Remote{}),
		config.Remote{Name: "remote1"},
	)

	manager.updateResourceRemoteNames()

	test.That(
		t,
		manager.resourceRemoteNames,
		test.ShouldResemble,
		map[resource.Name]string{arm.Named("arm1_r1"): "remote1", arm.Named("arm2_r1"): "remote1"},
	)
}

func TestManagerWithSameNameInRemoteNoPrefix(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{Name: "remote1", Prefix: false}),
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{Name: "remote2", Prefix: false}),
		config.Remote{Name: "remote2"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, err, test.ShouldBeError,
		errors.Errorf("multiple remote resources with name %q. Change duplicate names to access", arm.Named("arm1_r1")))
}

func TestManagerWithSameNameInRemoteOneWithPrefix(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{
			Name:   "remote1",
			Prefix: true,
		}),
		config.Remote{
			Name:   "remote1",
			Prefix: true,
		},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)

	_, err := manager.ResourceByName(arm.Named("remote1.arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote2.arm1_r1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("remote2.arm1_r1")))
	_, err = manager.ResourceByName(arm.Named("remote1.arm1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("remote1.arm1")))
	_, err = manager.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerWithSameNameInRemoteBothWithPrefix(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{
			Name:   "remote1",
			Prefix: true,
		}),
		config.Remote{
			Name:   "remote1",
			Prefix: true,
		},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{
			Name:   "remote2",
			Prefix: true,
		}),
		config.Remote{
			Name:   "remote2",
			Prefix: true,
		},
	)

	_, err := manager.ResourceByName(arm.Named("remote1.arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote2.arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1.arm1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("remote1.arm1")))
	_, err = manager.ResourceByName(arm.Named("remote2.arm1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("remote2.arm1")))
	_, err = manager.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("arm1_r1")))
	_, err = manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
}

func TestManagerWithSameNameInBaseAndRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, ""), config.Remote{}),
		config.Remote{Name: "remote1"},
	)

	_, err := manager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.ResourceByName(arm.Named("remote1.arm1"))
	test.That(t, err, test.ShouldBeError, errors.Errorf("resource %q not found", arm.Named("remote1.arm1")))
}

func TestManagerMergeNamesWithRemotesDedupe(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), manager), test.ShouldBeNil)
	}()
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1")...)

	test.That(
		t,
		utils.NewStringSet(manager.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(manager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
}

func TestManagerClone(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	manager := managerForRemoteRobot(injectRobot)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(context.Background(), setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := manager.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	newManager := manager.Clone()
	defer func() {
		test.That(t, utils.TryClose(context.Background(), newManager), test.ShouldBeNil)
	}()

	// remove and delete manager to prove clone
	delete(manager.remotes, "remote1")
	manager.remotes = nil
	manager.resources.Remove(arm.Named("arm1"))
	manager.resources.Remove(camera.Named("camera1"))
	manager.resources.Remove(gripper.Named("gripper1"))
	manager.resources.Remove(servo.Named("servo1"))
	manager.resources = nil

	_, ok := manager.processManager.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	err = manager.processManager.Stop()
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(newManager.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(newManager.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(newManager.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)

	_, err = newManager.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(arm.Named("arm1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(arm.Named("arm1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(base.Named("base1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(base.Named("base1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(base.Named("base1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(board.Named("board1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(board.Named("board1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(board.Named("board1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(camera.Named("camera1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(camera.Named("camera1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(camera.Named("camera1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(camera.Named("camera1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(gripper.Named("gripper1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(gripper.Named("gripper1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(gripper.Named("gripper1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(gripper.Named("gripper1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(motor.Named("motor1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(motor.Named("motor1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(motor.Named("motor1_what"))
	test.That(t, err, test.ShouldBeError)

	_, err = newManager.ResourceByName(servo.Named("servo1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(servo.Named("servo1_r1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(servo.Named("servo1_r2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = newManager.ResourceByName(servo.Named("servo1_what"))
	test.That(t, err, test.ShouldBeError)

	proc, ok := newManager.processManager.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc.ID(), test.ShouldEqual, "1")
	proc, ok = newManager.processManager.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc.ID(), test.ShouldEqual, "2")
	_, ok = newManager.processManager.ProcessByID("what")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestManagerAdd(t *testing.T) {
	logger := golog.NewTestLogger(t)
	manager := newResourceManager(resourceManagerOptions{}, logger)

	injectArm := &inject.Arm{}
	cfg := &config.Component{Type: arm.SubtypeName, Name: "arm1"}
	rName := cfg.ResourceName()
	manager.addResource(rName, injectArm)
	arm1, err := manager.ResourceByName(rName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, arm1, test.ShouldEqual, injectArm)

	injectBoard := &inject.Board{}
	injectBoard.SPINamesFunc = func() []string {
		return []string{"spi1"}
	}
	injectBoard.I2CNamesFunc = func() []string {
		return []string{"i2c1"}
	}
	injectBoard.AnalogReaderNamesFunc = func() []string {
		return []string{"analog1"}
	}
	injectBoard.DigitalInterruptNamesFunc = func() []string {
		return []string{"digital1"}
	}
	injectBoard.SPIByNameFunc = func(name string) (board.SPI, bool) {
		return &inject.SPI{}, true
	}
	injectBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		return &inject.I2C{}, true
	}
	injectBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
		return &fakeboard.Analog{}, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return &board.BasicDigitalInterrupt{}, true
	}

	cfg = &config.Component{Type: board.SubtypeName, Name: "board1"}
	rName = cfg.ResourceName()
	manager.addResource(rName, injectBoard)
	board1, err := manager.ResourceByName(board.Named("board1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, board1, test.ShouldEqual, injectBoard)
	resource1, err := manager.ResourceByName(rName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resource1, test.ShouldEqual, injectBoard)

	injectMotionService := &inject.MotionService{}
	injectMotionService.MoveFunc = func(
		ctx context.Context,
		componentName resource.Name,
		grabPose *referenceframe.PoseInFrame,
		worldState *commonpb.WorldState,
	) (bool, error) {
		return false, nil
	}
	objectMResName := motion.Name
	manager.addResource(objectMResName, injectMotionService)
	motionService, err := manager.ResourceByName(objectMResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, motionService, test.ShouldEqual, injectMotionService)

	injectVisionService := &inject.VisionService{}
	injectVisionService.GetObjectPointCloudsFunc = func(
		ctx context.Context,
		cameraName, segmenterName string,
		parameters config.AttributeMap,
	) ([]*viz.Object, error) {
		return []*viz.Object{viz.NewEmptyObject()}, nil
	}
	objectSegResName := vision.Name
	manager.addResource(objectSegResName, injectVisionService)
	objectSegmentationService, err := manager.ResourceByName(objectSegResName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objectSegmentationService, test.ShouldEqual, injectVisionService)
}

func TestManagerNewComponent(t *testing.T) {
	cfg := &config.Config{
		Components: []config.Component{
			{
				Name:      "arm1",
				Model:     "fake",
				Type:      arm.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "arm2",
				Model:     "fake",
				Type:      arm.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "arm3",
				Model:     "fake",
				Type:      arm.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "base1",
				Model:     "fake",
				Type:      base.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "base2",
				Model:     "fake",
				Type:      base.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "base3",
				Model:     "fake",
				Type:      base.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:                "board1",
				Model:               "fake",
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board2",
				Model:               "fake",
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:                "board3",
				Model:               "fake",
				Type:                board.SubtypeName,
				ConvertedAttributes: &board.Config{},
				DependsOn:           []string{},
			},
			{
				Name:      "camera1",
				Model:     "fake",
				Type:      camera.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "camera2",
				Model:     "fake",
				Type:      camera.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "camera3",
				Model:     "fake",
				Type:      camera.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "gripper1",
				Model:     "fake",
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm1", "camera1"},
			},
			{
				Name:      "gripper2",
				Model:     "fake",
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm2", "camera2"},
			},
			{
				Name:      "gripper3",
				Model:     "fake",
				Type:      gripper.SubtypeName,
				DependsOn: []string{"arm3", "camera3"},
			},
			{
				Name:                "inputController1",
				Model:               "fake",
				Type:                input.SubtypeName,
				ConvertedAttributes: &fake.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "inputController2",
				Model:               "fake",
				Type:                input.SubtypeName,
				ConvertedAttributes: &fake.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "inputController3",
				Model:               "fake",
				Type:                input.SubtypeName,
				ConvertedAttributes: &fake.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:                "motor1",
				Model:               "fake",
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board1"},
			},
			{
				Name:                "motor2",
				Model:               "fake",
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board2"},
			},
			{
				Name:                "motor3",
				Model:               "fake",
				Type:                motor.SubtypeName,
				ConvertedAttributes: &motor.Config{},
				DependsOn:           []string{"board3"},
			},
			{
				Name:      "sensor1",
				Model:     "fake",
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "sensor2",
				Model:     "fake",
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "sensor3",
				Model:     "fake",
				Type:      sensor.SubtypeName,
				DependsOn: []string{"board3"},
			},
			{
				Name:      "servo1",
				Model:     "fake",
				Type:      servo.SubtypeName,
				DependsOn: []string{"board1"},
			},
			{
				Name:      "servo2",
				Model:     "fake",
				Type:      servo.SubtypeName,
				DependsOn: []string{"board2"},
			},
			{
				Name:      "servo3",
				Model:     "fake",
				Type:      servo.SubtypeName,
				DependsOn: []string{"board3"},
			},
		},
	}
	logger := golog.NewTestLogger(t)
	robotForRemote := &localRobot{
		manager: newResourceManager(resourceManagerOptions{}, logger),
		logger:  logger,
		config:  cfg,
	}
	test.That(t, robotForRemote.manager.newComponents(context.Background(),
		cfg.Components, robotForRemote), test.ShouldBeNil)
	robotForRemote.config.Components[8].DependsOn = append(robotForRemote.config.Components[8].DependsOn, "arm3")
	robotForRemote.manager = newResourceManager(resourceManagerOptions{}, logger)
	err := robotForRemote.manager.newComponents(context.Background(),
		robotForRemote.config.Components, robotForRemote)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual,
		"circular dependency - \"arm3\" already depends on \"board3\"")
}

func TestManagerFilterFromConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	ctx, cancel := context.WithCancel(context.Background())

	manager := managerForRemoteRobot(injectRobot)
	defer func() {
		test.That(t, utils.TryClose(ctx, manager), test.ShouldBeNil)
	}()
	defer cancel()
	manager.addRemote(context.Background(),
		newRemoteRobot(ctx, setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	manager.addRemote(context.Background(),
		newRemoteRobot(ctx, setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := manager.processManager.AddProcess(ctx, &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = manager.processManager.AddProcess(ctx, &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkEmpty := func(toCheck *resourceManager) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, toCheck.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}

	filtered, err := manager.FilterFromConfig(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "what",
			},
		},
		Components: []config.Component{
			{
				Name: "what1",
				Type: arm.SubtypeName,
			},
			{
				Name: "what5",
				Type: base.SubtypeName,
			},
			{
				Name: "what3",
				Type: board.SubtypeName,
			},
			{
				Name: "what4",
				Type: camera.SubtypeName,
			},
			{
				Name: "what5",
				Type: gripper.SubtypeName,
			},
			{
				Name: "what6",
				Type: motor.SubtypeName,
			},
			{
				Name: "what7",
				Type: sensor.SubtypeName,
			},
			{
				Name: "what8",
				Type: servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "what",
				Name: "echo",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Components: []config.Component{
			{
				Name: "what1",
				Type: "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	cloned := manager.Clone()
	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Components: []config.Component{
			{
				Name: "arm2",
				Type: arm.SubtypeName,
			},
			{
				Name: "base2",
				Type: base.SubtypeName,
			},
			{
				Name: "board2",
				Type: board.SubtypeName,
			},
			{
				Name: "camera2",
				Type: camera.SubtypeName,
			},
			{
				Name: "gripper2",
				Type: gripper.SubtypeName,
			},
			{
				Name: "inputController2",
				Type: input.SubtypeName,
			},
			{
				Name: "motor2",
				Type: motor.SubtypeName,
			},
			{
				Name: "sensor2",
				Type: sensor.SubtypeName,
			},

			{
				Name: "servo2",
				Type: servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm2")}
	baseNames := []resource.Name{base.Named("base2")}
	boardNames := []resource.Name{board.Named("board2")}
	cameraNames := []resource.Name{camera.Named("camera2")}
	gripperNames := []resource.Name{gripper.Named("gripper2")}
	inputNames := []resource.Name{input.Named("inputController2")}
	motorNames := []resource.Name{motor.Named("motor2")}
	servoNames := []resource.Name{servo.Named("servo2")}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldBeEmpty)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	manager = cloned.Clone()

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote2",
			},
		},
		Components: []config.Component{
			{
				Name: "arm2",
				Type: arm.SubtypeName,
			},
			{
				Name: "base2",
				Type: base.SubtypeName,
			},
			{
				Name: "board2",
				Type: board.SubtypeName,
			},
			{
				Name: "camera2",
				Type: camera.SubtypeName,
			},
			{
				Name: "gripper2",
				Type: gripper.SubtypeName,
			},
			{
				Name: "inputController2",
				Type: input.SubtypeName,
			},
			{
				Name: "motor2",
				Type: motor.SubtypeName,
			},
			{
				Name: "sensor2",
				Type: sensor.SubtypeName,
			},
			{
				Name: "servo2",
				Type: servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
	baseNames = []resource.Name{
		base.Named("base2"),
		base.Named("base1_r2"),
		base.Named("base2_r2"),
	}
	boardNames = []resource.Name{
		board.Named("board2"),
		board.Named("board1_r2"),
		board.Named("board2_r2"),
	}
	cameraNames = []resource.Name{
		camera.Named("camera2"),
		camera.Named("camera1_r2"),
		camera.Named("camera2_r2"),
	}
	gripperNames = []resource.Name{
		gripper.Named("gripper2"),
		gripper.Named("gripper1_r2"),
		gripper.Named("gripper2_r2"),
	}
	inputNames = []resource.Name{
		input.Named("inputController2"),
		input.Named("inputController1_r2"),
		input.Named("inputController2_r2"),
	}
	motorNames = []resource.Name{
		motor.Named("motor2"),
		motor.Named("motor1_r2"),
		motor.Named("motor2_r2"),
	}
	servoNames = []resource.Name{
		servo.Named("servo2"),
		servo.Named("servo1_r2"),
		servo.Named("servo2_r2"),
	}

	test.That(
		t,
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	manager = cloned.Clone()

	filtered, err = manager.FilterFromConfig(ctx, &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote1",
			},
			{
				Name: "remote2",
			},
			{
				Name: "remote3",
			},
		},
		Components: []config.Component{
			{
				Name: "arm1",
				Type: arm.SubtypeName,
			},
			{
				Name: "arm2",
				Type: arm.SubtypeName,
			},
			{
				Name: "arm3",
				Type: arm.SubtypeName,
			},
			{
				Name: "base1",
				Type: base.SubtypeName,
			},
			{
				Name: "base2",
				Type: base.SubtypeName,
			},
			{
				Name: "base3",
				Type: base.SubtypeName,
			},
			{
				Name: "board1",
				Type: board.SubtypeName,
			},
			{
				Name: "board2",
				Type: board.SubtypeName,
			},
			{
				Name: "board3",
				Type: board.SubtypeName,
			},
			{
				Name: "camera1",
				Type: camera.SubtypeName,
			},
			{
				Name: "camera2",
				Type: camera.SubtypeName,
			},
			{
				Name: "camera3",
				Type: camera.SubtypeName,
			},
			{
				Name: "gripper1",
				Type: gripper.SubtypeName,
			},
			{
				Name: "gripper2",
				Type: gripper.SubtypeName,
			},
			{
				Name: "gripper3",
				Type: gripper.SubtypeName,
			},
			{
				Name: "inputController1",
				Type: input.SubtypeName,
			},
			{
				Name: "inputController2",
				Type: input.SubtypeName,
			},
			{
				Name: "inputController3",
				Type: input.SubtypeName,
			},
			{
				Name: "motor1",
				Type: motor.SubtypeName,
			},
			{
				Name: "motor2",
				Type: motor.SubtypeName,
			},
			{
				Name: "motor3",
				Type: motor.SubtypeName,
			},
			{
				Name: "sensor1",
				Type: sensor.SubtypeName,
			},
			{
				Name: "sensor2",
				Type: sensor.SubtypeName,
			},
			{
				Name: "sensor3",
				Type: sensor.SubtypeName,
			},
			{
				Name: "servo1",
				Type: servo.SubtypeName,
			},
			{
				Name: "servo2",
				Type: servo.SubtypeName,
			},
			{
				Name: "servo3",
				Type: servo.SubtypeName,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "1",
				Name: "echo", // does not matter
			},
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
			{
				ID:   "3",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)
	boardNames = []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	cameraNames = []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	gripperNames = []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	inputNames = []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	motorNames = []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	servoNames = []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			cameraNames,
			gripperNames,
			inputNames,
			motorNames,
			servoNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)
}

type fakeProcess struct {
	id string
}

func (fp *fakeProcess) ID() string {
	return fp.id
}

func (fp *fakeProcess) Start(ctx context.Context) error {
	return nil
}

func (fp *fakeProcess) Stop() error {
	return nil
}
