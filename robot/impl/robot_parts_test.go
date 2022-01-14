package robotimpl

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
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
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	functionvm "go.viam.com/rdk/function/vm"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/objectmanipulation"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestPartsForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}

	test.That(t, parts.RemoteNames(), test.ShouldBeEmpty)
	test.That(
		t,
		utils.NewStringSet(parts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board2"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func1", "func2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			inputNames,
			baseNames,
		)...),
	)

	_, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BaseByName("base1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.GripperByName("gripper1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.CameraByName("camera1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ServoByName("servo1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.MotorByName("motor1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.InputControllerByName("inputController1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(arm.Named("arm_what"))
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.ResourceByName(servo.Named("servo1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(servo.Named("servo_what"))
	test.That(t, ok, test.ShouldBeFalse)
}

func TestPartsMergeNamesWithRemotes(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(parts.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)

	_, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BaseByName("base1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BaseByName("base1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BaseByName("base1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.GripperByName("gripper1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.GripperByName("gripper1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.GripperByName("gripper1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.CameraByName("camera1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.CameraByName("camera1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.CameraByName("camera1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BoardByName("board1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BoardByName("board1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.SensorByName("sensor1_r1")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.SensorByName("sensor1_r2")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.SensorByName("sensor1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ServoByName("servo1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ServoByName("servo1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ServoByName("servo1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.MotorByName("motor1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.MotorByName("motor1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.MotorByName("motor1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.InputControllerByName("inputController1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.InputControllerByName("inputController1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.InputControllerByName("inputController1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(arm.Named("arm1_r2"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(arm.Named("arm1_what"))
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = parts.ResourceByName(servo.Named("servo1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(servo.Named("servo1_r1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(servo.Named("servo1_r2"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ResourceByName(servo.Named("servo1_what"))
	test.That(t, ok, test.ShouldBeFalse)
}

func TestPartsMergeNamesWithRemotesDedupe(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1")...)

	test.That(
		t,
		utils.NewStringSet(parts.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
}

func TestPartsClone(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	newParts := parts.Clone()

	// remove and delete parts to prove clone
	delete(parts.remotes, "remote1")
	parts.remotes = nil
	delete(parts.sensors, "sensor1")
	parts.sensors = nil
	delete(parts.functions, "func1")
	parts.functions = nil
	delete(parts.resources, arm.Named("arm1"))
	delete(parts.resources, servo.Named("servo1"))
	delete(parts.resources, gripper.Named("gripper1"))
	delete(parts.resources, camera.Named("camera1"))
	parts.resources = nil

	_, ok := parts.processManager.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	parts.processManager.Stop()

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(newParts.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(newParts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(newParts.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)

	_, ok = newParts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BaseByName("base1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BaseByName("base1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BaseByName("base1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.GripperByName("gripper1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.GripperByName("gripper1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.GripperByName("gripper1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.CameraByName("camera1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.CameraByName("camera1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.CameraByName("camera1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BoardByName("board1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BoardByName("board1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ServoByName("servo1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ServoByName("servo1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ServoByName("servo1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.MotorByName("motor1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.MotorByName("motor1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.MotorByName("motor1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.InputControllerByName("inputController1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.InputControllerByName("inputController1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.InputControllerByName("inputController1_what")
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.ResourceByName(arm.Named("arm1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(arm.Named("arm1_r1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(arm.Named("arm1_r2"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(arm.Named("arm1_what"))
	test.That(t, ok, test.ShouldBeFalse)

	_, ok = newParts.ResourceByName(servo.Named("servo1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(servo.Named("servo1_r1"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(servo.Named("servo1_r2"))
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ResourceByName(servo.Named("servo1_what"))
	test.That(t, ok, test.ShouldBeFalse)

	proc, ok := newParts.processManager.ProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc.ID(), test.ShouldEqual, "1")
	proc, ok = newParts.processManager.ProcessByID("2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, proc.ID(), test.ShouldEqual, "2")
	_, ok = newParts.processManager.ProcessByID("what")
	test.That(t, ok, test.ShouldBeFalse)
}

func TestPartsAdd(t *testing.T) {
	logger := golog.NewTestLogger(t)
	parts := newRobotParts(logger)

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

	cfg := &config.Component{Type: config.ComponentTypeBoard, Name: "board1"}
	rName := cfg.ResourceName()
	parts.addResource(rName, injectBoard)
	board1, ok := parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1, test.ShouldEqual, injectBoard)
	resource1, ok := parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectBoard)

	injectBase := &inject.Base{}
	cfg = &config.Component{Type: config.ComponentTypeBase, Name: "base1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectBase)
	base1, ok := parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1, test.ShouldEqual, injectBase)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectBase)

	injectSensor := &inject.Sensor{}
	parts.AddSensor(injectSensor, config.Component{Name: "sensor1"})
	sensor1, ok := parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxySensor).actual, test.ShouldEqual, injectSensor)
	parts.AddSensor(sensor1, config.Component{Name: "sensor1"})
	test.That(t, sensor1.(*proxySensor).actual, test.ShouldEqual, injectSensor)

	injectObjectManipulationService := &inject.ObjectManipulationService{}
	injectObjectManipulationService.DoGrabFunc = func(
		ctx context.Context,
		gripperName,
		armName,
		cameraName string,
		cameraPoint *r3.Vector) (bool, error) {
		return false, nil
	}
	objectMResName := objectmanipulation.Name
	parts.addResource(objectMResName, injectObjectManipulationService)
	objectManipulationService, ok := parts.ResourceByName(objectMResName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, objectManipulationService, test.ShouldEqual, injectObjectManipulationService)

	injectArm := &inject.Arm{}
	cfg = &config.Component{Type: config.ComponentTypeArm, Name: "arm1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectArm)
	arm1, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, arm1, test.ShouldEqual, injectArm)
	parts.addResource(rName, arm1)
	test.That(t, arm1, test.ShouldEqual, injectArm)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectArm)

	injectMotor := &inject.Motor{}
	cfg = &config.Component{Type: config.ComponentTypeMotor, Name: "motor1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectMotor)
	motor1, ok := parts.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motor1, test.ShouldEqual, injectMotor)
	parts.addResource(rName, motor1)
	test.That(t, motor1, test.ShouldEqual, injectMotor)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectMotor)

	injectServo := &inject.Servo{}
	cfg = &config.Component{Type: config.ComponentTypeServo, Name: "servo1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectServo)
	servo1, ok := parts.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, servo1, test.ShouldEqual, injectServo)
	parts.addResource(rName, servo1)
	test.That(t, servo1, test.ShouldEqual, injectServo)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectServo)

	injectGripper := &inject.Gripper{}
	cfg = &config.Component{Type: config.ComponentTypeGripper, Name: "gripper1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectGripper)
	gripper1, ok := parts.GripperByName("gripper1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, gripper1, test.ShouldEqual, injectGripper)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectGripper)

	injectCamera := &inject.Camera{}
	cfg = &config.Component{Type: config.ComponentTypeCamera, Name: "camera1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectCamera)
	camera1, ok := parts.CameraByName("camera1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, camera1, test.ShouldEqual, injectCamera)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectCamera)

	injectInputController := &inject.InputController{}
	cfg = &config.Component{Type: config.ComponentTypeInputController, Name: "inputController1"}
	rName = cfg.ResourceName()
	parts.addResource(rName, injectInputController)
	inputController1, ok := parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1, test.ShouldEqual, injectInputController)
	resource1, ok = parts.ResourceByName(rName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resource1, test.ShouldEqual, injectInputController)
}

func TestPartsMergeAdd(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	//nolint:dupl
	checkEmpty := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.MotorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.InputControllerNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.FunctionNames()...), test.ShouldBeEmpty)
		test.That(t, toCheck.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}
	checkSame := func(toCheck *robotParts) {
		t.Helper()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
		gripperNames = append(
			gripperNames,
			rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
		cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
		servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
		motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
		motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
		inputNames := []resource.Name{
			input.Named("inputController1"),
			input.Named("inputController2"),
		}
		inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

		test.That(
			t,
			utils.NewStringSet(toCheck.RemoteNames()...),
			test.ShouldResemble,
			utils.NewStringSet("remote1", "remote2"),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ArmNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.GripperNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.CameraNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BaseNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BoardNames()...),
			test.ShouldResemble,
			utils.NewStringSet(
				"board1",
				"board2",
				"board1_r1",
				"board2_r1",
				"board1_r2",
				"board2_r2",
			),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.SensorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ServoNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.MotorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.InputControllerNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.FunctionNames()...),
			test.ShouldResemble,
			utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
		)
		test.That(
			t,
			rdktestutils.NewResourceNameSet(toCheck.ResourceNames()...),
			test.ShouldResemble,
			rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
				armNames,
				boardNames,
				gripperNames,
				cameraNames,
				servoNames,
				motorNames,
				inputNames,
				inputNames,
				baseNames,
			)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.processManager.ProcessIDs()...),
			test.ShouldResemble,
			utils.NewStringSet("1", "2"),
		)
	}
	result, err := parts.MergeAdd(newRobotParts(logger))
	test.That(t, err, test.ShouldBeNil)
	checkSame(parts)

	emptyParts := newRobotParts(logger)
	test.That(t, result.Process(context.Background(), emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_other1"), config.Remote{}),
		config.Remote{Name: "other1"},
	)
	result, err = parts.MergeAdd(otherParts)
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(
		armNames,
		rdktestutils.AddSuffixes(armNames, "_r1", "_r2", "_other", "_other1")...)
	boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(
		boardNames,
		rdktestutils.AddSuffixes(boardNames, "_r1", "_r2", "_other", "_other1")...)
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(
		gripperNames,
		rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2", "_other", "_other1")...)
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(
		cameraNames,
		rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2", "_other", "_other1")...)
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(
		servoNames,
		rdktestutils.AddSuffixes(servoNames, "_r1", "_r2", "_other", "_other1")...)
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(
		motorNames,
		rdktestutils.AddSuffixes(motorNames, "_r1", "_r2", "_other", "_other1")...)
	inputNames := []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(
		inputNames,
		rdktestutils.AddSuffixes(inputNames, "_r1", "_r2", "_other", "_other1")...)
	baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(
		baseNames,
		rdktestutils.AddSuffixes(baseNames, "_r1", "_r2", "_other", "_other1")...)

	test.That(
		t,
		utils.NewStringSet(parts.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2", "other1"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(
			"board1",
			"board2",
			"board1_r1",
			"board2_r1",
			"board1_r2",
			"board2_r2",
			"board1_other",
			"board2_other",
			"board1_other1",
			"board2_other1",
		),
	)
	test.That(
		t,
		utils.NewStringSet(parts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet(
			"func1",
			"func2",
			"func1_r1",
			"func2_r1",
			"func1_r2",
			"func2_r2",
			"func1_other",
			"func2_other",
			"func1_other1",
			"func2_other1",
		),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)

	emptyParts = newRobotParts(logger)
	test.That(t, result.Process(context.Background(), emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	sameParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	result, err = parts.MergeAdd(sameParts)
	test.That(t, err, test.ShouldBeNil)

	test.That(
		t,
		utils.NewStringSet(parts.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2", "other1"),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet(
			"board1",
			"board2",
			"board1_r1",
			"board2_r1",
			"board1_r2",
			"board2_r2",
			"board1_other",
			"board2_other",
			"board1_other1",
			"board2_other1",
		),
	)
	test.That(
		t,
		utils.NewStringSet(parts.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(parts.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet(
			"func1",
			"func2",
			"func1_r1",
			"func2_r1",
			"func1_r2",
			"func2_r2",
			"func1_other",
			"func2_other",
			"func1_other1",
			"func2_other1",
		),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(parts.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)

	emptyParts = newRobotParts(logger)
	test.That(t, result.Process(context.Background(), emptyParts), test.ShouldBeNil)
	test.That(t, utils.NewStringSet(emptyParts.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ArmNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.GripperNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.CameraNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ServoNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.MotorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.InputControllerNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.FunctionNames()...), test.ShouldBeEmpty)
	test.That(t, emptyParts.ResourceNames(), test.ShouldBeEmpty)
	test.That(
		t,
		utils.NewStringSet(emptyParts.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("1", "2"),
	)

	err = result.Process(context.Background(), parts)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected")
}

func TestPartsMergeModify(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkSame := func(toCheck *robotParts) {
		t.Helper()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
		gripperNames = append(
			gripperNames,
			rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
		cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
		servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
		motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
		motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
		inputNames := []resource.Name{
			input.Named("inputController1"),
			input.Named("inputController2"),
		}
		inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

		test.That(
			t,
			utils.NewStringSet(toCheck.RemoteNames()...),
			test.ShouldResemble,
			utils.NewStringSet("remote1", "remote2"),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ArmNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.GripperNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.CameraNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BaseNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BoardNames()...),
			test.ShouldResemble,
			utils.NewStringSet(
				"board1",
				"board2",
				"board1_r1",
				"board2_r1",
				"board1_r2",
				"board2_r2",
			),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.SensorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ServoNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.MotorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.InputControllerNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.FunctionNames()...),
			test.ShouldResemble,
			utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
		)
		test.That(
			t,
			rdktestutils.NewResourceNameSet(parts.ResourceNames()...),
			test.ShouldResemble,
			rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
				armNames,
				boardNames,
				gripperNames,
				cameraNames,
				servoNames,
				motorNames,
				inputNames,
				baseNames,
			)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.processManager.ProcessIDs()...),
			test.ShouldResemble,
			utils.NewStringSet("1", "2"),
		)

		board1, ok := toCheck.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		board2r1, ok := toCheck.BoardByName("board2_r1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(
			t,
			utils.NewStringSet(board1.AnalogReaderNames()...),
			test.ShouldResemble,
			utils.NewStringSet("analog1", "analog2"),
		)
		test.That(
			t,
			utils.NewStringSet(board1.DigitalInterruptNames()...),
			test.ShouldResemble,
			utils.NewStringSet("digital1", "digital2"),
		)
		test.That(
			t,
			utils.NewStringSet(board2r1.AnalogReaderNames()...),
			test.ShouldResemble,
			utils.NewStringSet("analog1", "analog2"),
		)
		test.That(
			t,
			utils.NewStringSet(board2r1.DigitalInterruptNames()...),
			test.ShouldResemble,
			utils.NewStringSet("digital1", "digital2"),
		)
	}
	result, err := parts.MergeModify(context.Background(), newRobotParts(logger), &config.Diff{})
	test.That(t, err, test.ShouldBeNil)
	checkSame(parts)

	emptyParts := newRobotParts(logger)
	test.That(t, result.Process(context.Background(), emptyParts), test.ShouldBeNil)
	test.That(t, utils.NewStringSet(emptyParts.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ArmNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.GripperNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.CameraNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ServoNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.MotorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.InputControllerNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.FunctionNames()...), test.ShouldBeEmpty)
	test.That(t, emptyParts.ResourceNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldBeEmpty)

	test.That(t, result.Process(context.Background(), parts), test.ShouldBeNil)

	replacementParts := newRobotParts(logger)
	robotForRemote := &localRobot{parts: newRobotParts(logger), logger: logger}

	robotForRemote.parts.addFunction("func2_r1")

	cfg := config.Component{Type: config.ComponentTypeArm, Name: "arm2_r1"}
	rName := cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Arm{})

	cfg = config.Component{Type: config.ComponentTypeBase, Name: "base2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Base{})

	cfg = config.Component{Type: config.ComponentTypeBoard, Name: "board2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Board{})

	cfg = config.Component{Type: config.ComponentTypeMotor, Name: "motor2_r1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Motor{})

	cfg = config.Component{Type: config.ComponentTypeServo, Name: "servo2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Servo{})

	cfg = config.Component{Type: config.ComponentTypeGripper, Name: "gripper2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Gripper{})

	cfg = config.Component{Type: config.ComponentTypeCamera, Name: "camera2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Camera{})

	cfg = config.Component{Type: config.ComponentTypeInputController, Name: "inputController2_r1"}
	rName = cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.InputController{})

	remote1Replacemenet := newRemoteRobot(robotForRemote, config.Remote{Name: "remote1"})
	replacementParts.addRemote(remote1Replacemenet, config.Remote{Name: "remote1"})

	cfg = config.Component{Type: config.ComponentTypeArm, Name: "arm1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Arm{})

	cfg = config.Component{Type: config.ComponentTypeBase, Name: "base1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Base{})

	cfg = config.Component{Type: config.ComponentTypeBoard, Name: "board1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Board{})

	cfg = config.Component{Type: config.ComponentTypeMotor, Name: "motor1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Motor{})

	cfg = config.Component{Type: config.ComponentTypeServo, Name: "servo1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Servo{})

	cfg = config.Component{Type: config.ComponentTypeGripper, Name: "gripper1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Gripper{})

	cfg = config.Component{Type: config.ComponentTypeCamera, Name: "camera1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Camera{})

	cfg = config.Component{Type: config.ComponentTypeInputController, Name: "inputController1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.InputController{})

	fp1 := &fakeProcess{id: "1"}
	_, err = replacementParts.processManager.AddProcess(context.Background(), fp1, false)
	test.That(t, err, test.ShouldBeNil)
}

func TestPartsMergeRemove(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkSame := func(toCheck *robotParts) {
		t.Helper()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
		gripperNames = append(
			gripperNames,
			rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
		cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
		servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
		motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
		motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
		inputNames := []resource.Name{
			input.Named("inputController1"),
			input.Named("inputController2"),
		}
		inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

		test.That(
			t,
			utils.NewStringSet(toCheck.RemoteNames()...),
			test.ShouldResemble,
			utils.NewStringSet("remote1", "remote2"),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ArmNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.GripperNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.CameraNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BaseNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.BoardNames()...),
			test.ShouldResemble,
			utils.NewStringSet(
				"board1",
				"board2",
				"board1_r1",
				"board2_r1",
				"board1_r2",
				"board2_r2",
			),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.SensorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.ServoNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.MotorNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.InputControllerNames()...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.FunctionNames()...),
			test.ShouldResemble,
			utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
		)
		test.That(
			t,
			rdktestutils.NewResourceNameSet(toCheck.ResourceNames()...),
			test.ShouldResemble,
			rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
				armNames,
				boardNames,
				gripperNames,
				cameraNames,
				servoNames,
				motorNames,
				inputNames,
				baseNames,
			)...),
		)
		test.That(
			t,
			utils.NewStringSet(toCheck.processManager.ProcessIDs()...),
			test.ShouldResemble,
			utils.NewStringSet("1", "2"),
		)
	}

	parts.MergeRemove(newRobotParts(logger))
	checkSame(parts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_other1"), config.Remote{}),
		config.Remote{Name: "other1"},
	)
	parts.MergeRemove(otherParts)
	checkSame(parts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	sameParts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	parts.MergeRemove(sameParts)
	checkSame(sameParts)
	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.ServoNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.MotorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.InputControllerNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.FunctionNames()...), test.ShouldBeEmpty)
	test.That(t, parts.ResourceNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldBeEmpty)
}

func TestPartsFilterFromConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}),
		config.Remote{Name: "remote1"},
	)
	parts.addRemote(
		newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}),
		config.Remote{Name: "remote2"},
	)
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	//nolint:dupl
	checkEmpty := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ServoNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.MotorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.InputControllerNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.FunctionNames()...), test.ShouldBeEmpty)
		test.That(t, toCheck.ResourceNames(), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}

	filtered, err := parts.FilterFromConfig(context.Background(), &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(context.Background(), &config.Config{
		Remotes: []config.Remote{
			{
				Name: "what",
			},
		},
		Components: []config.Component{
			{
				Name: "what1",
				Type: config.ComponentTypeArm,
			},
			{
				Name: "what2",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "what3",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "what5",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "what6",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "what7",
				Type: config.ComponentTypeBoard,
			},
			{

				Name: "what8",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "what9",
				Type: config.ComponentTypeMotor,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "what",
				Name: "echo",
			},
		},
		Functions: []functionvm.FunctionConfig{
			{
				Name: "what",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(context.Background(), &config.Config{
		Components: []config.Component{
			{
				Name: "what1",
				Type: "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(context.Background(), &config.Config{
		Components: []config.Component{
			{
				Name: "arm2",
				Type: config.ComponentTypeArm,
			},
			{
				Name: "gripper2",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "camera2",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "base2",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "sensor2",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "board2",
				Type: config.ComponentTypeBoard,
			},
			{
				Name: "servo2",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "motor2",
				Type: config.ComponentTypeMotor,
			},
			{
				Name: "inputController2",
				Type: config.ComponentTypeInputController,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
		Functions: []functionvm.FunctionConfig{
			{
				Name: "func2",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm2")}
	boardNames := []resource.Name{board.Named("board2")}
	gripperNames := []resource.Name{gripper.Named("gripper2")}
	cameraNames := []resource.Name{camera.Named("camera2")}
	servoNames := []resource.Name{servo.Named("servo2")}
	motorNames := []resource.Name{motor.Named("motor2")}
	inputNames := []resource.Name{input.Named("inputController2")}
	baseNames := []resource.Name{base.Named("base2")}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldBeEmpty)
	test.That(
		t,
		utils.NewStringSet(filtered.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board2"),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	filtered, err = parts.FilterFromConfig(context.Background(), &config.Config{
		Remotes: []config.Remote{
			{
				Name: "remote2",
			},
		},
		Components: []config.Component{
			{
				Name: "arm2",
				Type: config.ComponentTypeArm,
			},
			{
				Name: "gripper2",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "camera2",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "base2",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "sensor2",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "board2",
				Type: config.ComponentTypeBoard,
			},
			{
				Name: "servo2",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "motor2",
				Type: config.ComponentTypeMotor,
			},
			{
				Name: "inputController2",
				Type: config.ComponentTypeInputController,
			},
		},
		Processes: []pexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
		Functions: []functionvm.FunctionConfig{
			{
				Name: "func2",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm2"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
	boardNames = []resource.Name{
		board.Named("board2"),
		board.Named("board1_r2"),
		board.Named("board2_r2"),
	}
	gripperNames = []resource.Name{
		gripper.Named("gripper2"),
		gripper.Named("gripper1_r2"),
		gripper.Named("gripper2_r2"),
	}
	cameraNames = []resource.Name{
		camera.Named("camera2"),
		camera.Named("camera1_r2"),
		camera.Named("camera2_r2"),
	}
	servoNames = []resource.Name{
		servo.Named("servo2"),
		servo.Named("servo1_r2"),
		servo.Named("servo2_r2"),
	}
	motorNames = []resource.Name{
		motor.Named("motor2"),
		motor.Named("motor1_r2"),
		motor.Named("motor2_r2"),
	}
	inputNames = []resource.Name{
		input.Named("inputController2"),
		input.Named("inputController1_r2"),
		input.Named("inputController2_r2"),
	}
	baseNames = []resource.Name{
		base.Named("base2"),
		base.Named("base1_r2"),
		base.Named("base2_r2"),
	}

	test.That(
		t,
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote2"),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board2", "board1_r2", "board2_r2"),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func2", "func1_r2", "func2_r2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
		)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.processManager.ProcessIDs()...),
		test.ShouldResemble,
		utils.NewStringSet("2"),
	)

	filtered, err = parts.FilterFromConfig(context.Background(), &config.Config{
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
				Type: config.ComponentTypeArm,
			},
			{
				Name: "arm2",
				Type: config.ComponentTypeArm,
			},
			{
				Name: "arm3",
				Type: config.ComponentTypeArm,
			},
			{
				Name: "gripper1",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "gripper2",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "gripper3",
				Type: config.ComponentTypeGripper,
			},
			{
				Name: "camera1",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "camera2",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "camera3",
				Type: config.ComponentTypeCamera,
			},
			{
				Name: "base1",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "base2",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "base3",
				Type: config.ComponentTypeBase,
			},
			{
				Name: "sensor1",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "sensor2",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "sensor3",
				Type: config.ComponentTypeSensor,
			},
			{
				Name: "board1",
				Type: config.ComponentTypeBoard,
			},
			{
				Name: "board2",
				Type: config.ComponentTypeBoard,
			},
			{
				Name: "board3",
				Type: config.ComponentTypeBoard,
			},
			{
				Name: "servo1",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "servo2",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "servo3",
				Type: config.ComponentTypeServo,
			},
			{
				Name: "motor1",
				Type: config.ComponentTypeMotor,
			},
			{
				Name: "motor2",
				Type: config.ComponentTypeMotor,
			},
			{
				Name: "motor3",
				Type: config.ComponentTypeMotor,
			},
			{
				Name: "inputController1",
				Type: config.ComponentTypeInputController,
			},
			{
				Name: "inputController2",
				Type: config.ComponentTypeInputController,
			},
			{
				Name: "inputController3",
				Type: config.ComponentTypeInputController,
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
		Functions: []functionvm.FunctionConfig{
			{
				Name: "func1",
			},
			{
				Name: "func2",
			},
			{
				Name: "func3",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	armNames = append(armNames, rdktestutils.AddSuffixes(armNames, "_r1", "_r2")...)
	boardNames = []resource.Name{board.Named("board1"), board.Named("board2")}
	boardNames = append(boardNames, rdktestutils.AddSuffixes(boardNames, "_r1", "_r2")...)
	gripperNames = []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	gripperNames = append(gripperNames, rdktestutils.AddSuffixes(gripperNames, "_r1", "_r2")...)
	cameraNames = []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	cameraNames = append(cameraNames, rdktestutils.AddSuffixes(cameraNames, "_r1", "_r2")...)
	servoNames = []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	servoNames = append(servoNames, rdktestutils.AddSuffixes(servoNames, "_r1", "_r2")...)
	motorNames = []resource.Name{motor.Named("motor1"), motor.Named("motor2")}
	motorNames = append(motorNames, rdktestutils.AddSuffixes(motorNames, "_r1", "_r2")...)
	inputNames = []resource.Name{input.Named("inputController1"), input.Named("inputController2")}
	inputNames = append(inputNames, rdktestutils.AddSuffixes(inputNames, "_r1", "_r2")...)
	baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
	baseNames = append(baseNames, rdktestutils.AddSuffixes(baseNames, "_r1", "_r2")...)

	test.That(
		t,
		utils.NewStringSet(filtered.RemoteNames()...),
		test.ShouldResemble,
		utils.NewStringSet("remote1", "remote2"),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.ArmNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.GripperNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(gripperNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.CameraNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(cameraNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BaseNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.BoardNames()...),
		test.ShouldResemble,
		utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.SensorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.ServoNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(servoNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.MotorNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.InputControllerNames()...),
		test.ShouldResemble,
		utils.NewStringSet(rdktestutils.ExtractNames(inputNames...)...),
	)
	test.That(
		t,
		utils.NewStringSet(filtered.FunctionNames()...),
		test.ShouldResemble,
		utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"),
	)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(filtered.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(rdktestutils.ConcatResourceNames(
			armNames,
			boardNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
			inputNames,
			baseNames,
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
