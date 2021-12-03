package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/core/board"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/component/camera"
	"go.viam.com/core/component/gripper"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/component/servo"
	"go.viam.com/core/config"
	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/resource"
	"go.viam.com/core/robots/fake"
	coretestutils "go.viam.com/core/testutils"

	"go.viam.com/core/services"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestPartsForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2")}
	motorNames := []resource.Name{motor.Named("motor1"), motor.Named("motor2")}

	test.That(t, parts.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix"))
	test.That(t, utils.NewStringSet(parts.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(parts.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(parts.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2"))
	test.That(t, utils.NewStringSet(parts.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2"))
	test.That(t, coretestutils.NewResourceNameSet(parts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))

	_, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)
	base1, ok := parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
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
	lidar1, ok := parts.LidarByName("lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	_, ok = parts.LidarByName("lidar1_what")
	test.That(t, ok, test.ShouldBeFalse)
	board1, ok := parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1")
	_, ok = parts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)
	sensor1, ok := parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	_, ok = parts.SensorByName("sensor1_what")
	test.That(t, ok, test.ShouldBeFalse)
	fsm, ok := parts.SensorByName("forcematrix")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix")
	_, ok = parts.ServoByName("servo1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ServoByName("servo1_what")
	test.That(t, ok, test.ShouldBeFalse)
	_, ok = parts.MotorByName("motor1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.MotorByName("motor1_what")
	test.That(t, ok, test.ShouldBeFalse)
	inputController1, ok := parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1")
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
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
	motorNames := []resource.Name{
		motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"),
		motor.Named("motor1_r2"), motor.Named("motor2_r2"),
	}

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
	test.That(t, utils.NewStringSet(parts.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(parts.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(parts.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
	test.That(t, utils.NewStringSet(parts.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
	test.That(t, coretestutils.NewResourceNameSet(parts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))

	_, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = parts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	base1, ok := parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1, ok = parts.BaseByName("base1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1, ok = parts.BaseByName("base1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
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

	lidar1, ok := parts.LidarByName("lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1, ok = parts.LidarByName("lidar1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1, ok = parts.LidarByName("lidar1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	_, ok = parts.LidarByName("lidar1_what")
	test.That(t, ok, test.ShouldBeFalse)

	board1, ok := parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1")
	board1, ok = parts.BoardByName("board1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1_r1")
	board1, ok = parts.BoardByName("board1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1_r2")
	_, ok = parts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)

	sensor1, ok := parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1, ok = parts.SensorByName("sensor1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1, ok = parts.SensorByName("sensor1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
	fsm, ok := parts.SensorByName("forcematrix")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix")
	fsm, ok = parts.SensorByName("forcematrix_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix_r1")
	fsm, ok = parts.SensorByName("forcematrix_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix_r2")
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

	inputController1, ok := parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1")
	inputController1, ok = parts.InputControllerByName("inputController1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1_r1")
	inputController1, ok = parts.InputControllerByName("inputController1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1_r2")
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

func TestPartsClone(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	newParts := parts.Clone()

	// remove and delete parts to prove clone
	delete(parts.remotes, "remote1")
	parts.remotes = nil
	delete(parts.lidars, "lidar1")
	parts.lidars = nil
	delete(parts.bases, "base1")
	parts.bases = nil
	delete(parts.boards, "board1")
	parts.boards = nil
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

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
	motorNames := []resource.Name{
		motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"),
		motor.Named("motor1_r2"), motor.Named("motor2_r2"),
	}

	test.That(t, utils.NewStringSet(newParts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(newParts.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(newParts.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(newParts.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(newParts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(newParts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(newParts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(newParts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
	test.That(t, utils.NewStringSet(newParts.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(newParts.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(newParts.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
	test.That(t, utils.NewStringSet(newParts.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
	test.That(t, coretestutils.NewResourceNameSet(newParts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(newParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	_, ok = newParts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = newParts.ArmByName("arm1_what")
	test.That(t, ok, test.ShouldBeFalse)

	base1, ok := newParts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1, ok = newParts.BaseByName("base1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1, ok = newParts.BaseByName("base1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
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

	lidar1, ok := newParts.LidarByName("lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1, ok = newParts.LidarByName("lidar1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1, ok = newParts.LidarByName("lidar1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	_, ok = newParts.LidarByName("lidar1_what")
	test.That(t, ok, test.ShouldBeFalse)

	board1, ok := newParts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1")
	board1, ok = newParts.BoardByName("board1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1_r1")
	board1, ok = newParts.BoardByName("board1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual.(*fake.Board).Name, test.ShouldEqual, "board1_r2")
	_, ok = newParts.BoardByName("board1_what")
	test.That(t, ok, test.ShouldBeFalse)

	sensor1, ok := newParts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1, ok = newParts.SensorByName("sensor1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1, ok = newParts.SensorByName("sensor1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
	fsm, ok := newParts.SensorByName("forcematrix")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix")
	fsm, ok = newParts.SensorByName("forcematrix_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix_r1")
	fsm, ok = newParts.SensorByName("forcematrix_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual.(*fake.ForceMatrix).Name, test.ShouldEqual, "forcematrix_r2")
	_, ok = newParts.SensorByName("sensor1_what")
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

	inputController1, ok := newParts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1")
	inputController1, ok = newParts.InputControllerByName("inputController1_r1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1_r1")
	inputController1, ok = newParts.InputControllerByName("inputController1_r2")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual.(*fake.InputController).Name, test.ShouldEqual, "inputController1_r2")
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
		return &fake.Analog{}, true
	}
	injectBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return &board.BasicDigitalInterrupt{}, true
	}

	parts.AddBoard(injectBoard, config.Component{Name: "board1"})
	board1, ok := parts.BoardByName("board1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, board1.(*proxyBoard).actual, test.ShouldEqual, injectBoard)
	parts.AddBoard(board1, config.Component{Name: "board1"})
	test.That(t, board1.(*proxyBoard).actual, test.ShouldEqual, injectBoard)

	injectLidar := &inject.Lidar{}
	parts.AddLidar(injectLidar, config.Component{Name: "lidar1"})
	lidar1, ok := parts.LidarByName("lidar1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lidar1.(*proxyLidar).actual, test.ShouldEqual, injectLidar)
	parts.AddLidar(lidar1, config.Component{Name: "lidar1"})
	test.That(t, lidar1.(*proxyLidar).actual, test.ShouldEqual, injectLidar)

	injectBase := &inject.Base{}
	parts.AddBase(injectBase, config.Component{Name: "base1"})
	base1, ok := parts.BaseByName("base1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, base1.(*proxyBase).actual, test.ShouldEqual, injectBase)
	parts.AddBase(base1, config.Component{Name: "base1"})
	test.That(t, base1.(*proxyBase).actual, test.ShouldEqual, injectBase)

	injectSensor := &inject.Sensor{}
	parts.AddSensor(injectSensor, config.Component{Name: "sensor1"})
	sensor1, ok := parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxySensor).actual, test.ShouldEqual, injectSensor)
	parts.AddSensor(sensor1, config.Component{Name: "sensor1"})
	test.That(t, sensor1.(*proxySensor).actual, test.ShouldEqual, injectSensor)

	injectCompass := &inject.Compass{}
	parts.AddSensor(injectCompass, config.Component{Name: "sensor1"})
	sensor1, ok = parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyCompass).actual, test.ShouldEqual, injectCompass)
	parts.AddSensor(sensor1, config.Component{Name: "sensor1"})
	test.That(t, sensor1.(*proxyCompass).actual, test.ShouldEqual, injectCompass)

	injectRelativeCompass := &inject.RelativeCompass{}
	parts.AddSensor(injectRelativeCompass, config.Component{Name: "sensor1"})
	sensor1, ok = parts.SensorByName("sensor1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sensor1.(*proxyRelativeCompass).actual, test.ShouldEqual, injectRelativeCompass)
	parts.AddSensor(sensor1, config.Component{Name: "sensor1"})
	test.That(t, sensor1.(*proxyRelativeCompass).actual, test.ShouldEqual, injectRelativeCompass)

	injectFsm := &inject.ForceMatrix{}
	parts.AddSensor(injectFsm, config.Component{Name: "forcematrix"})
	fsm, ok := parts.SensorByName("forcematrix")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, fsm.(*proxyForceMatrix).actual, test.ShouldEqual, injectFsm)

	injectInputController := &inject.InputController{}
	parts.AddInputController(injectInputController, config.Component{Name: "inputController1"})
	inputController1, ok := parts.InputControllerByName("inputController1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, inputController1.(*proxyInputController).actual, test.ShouldEqual, injectInputController)
	parts.AddInputController(inputController1, config.Component{Name: "inputController1"})
	test.That(t, inputController1.(*proxyInputController).actual, test.ShouldEqual, injectInputController)

	injectObjectManipulationService := &inject.ObjectManipulationService{}
	injectObjectManipulationService.DoGrabFunc = func(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
		return false, nil
	}
	parts.AddService(injectObjectManipulationService, config.Service{Name: services.ObjectManipulationServiceName})
	objectManipulationService, ok := parts.ServiceByName(services.ObjectManipulationServiceName)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, objectManipulationService, test.ShouldEqual, injectObjectManipulationService)

	injectArm := &inject.Arm{}
	cfg := &config.Component{Type: config.ComponentTypeArm, Name: "arm1"}

	rName := cfg.ResourceName()
	parts.addResource(rName, injectArm)
	arm1, ok := parts.ArmByName("arm1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, arm1, test.ShouldEqual, injectArm)
	parts.addResource(rName, arm1)
	test.That(t, arm1, test.ShouldEqual, injectArm)
	resource1, ok := parts.ResourceByName(rName)
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
}

func TestPartsMergeAdd(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkEmpty := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldBeEmpty)
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
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
		motorNames := []resource.Name{
			motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"),
			motor.Named("motor1_r2"), motor.Named("motor2_r2"),
		}

		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
		test.That(t, utils.NewStringSet(toCheck.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
		test.That(t, utils.NewStringSet(toCheck.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
		test.That(t, utils.NewStringSet(toCheck.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
		test.That(t, utils.NewStringSet(toCheck.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
		test.That(t, coretestutils.NewResourceNameSet(toCheck.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
			armNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
		)...))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
	}
	result, err := parts.MergeAdd(newRobotParts(logger))
	test.That(t, err, test.ShouldBeNil)
	checkSame(parts)

	emptyParts := newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_other1"), config.Remote{}), config.Remote{Name: "other1"})
	result, err = parts.MergeAdd(otherParts)
	test.That(t, err, test.ShouldBeNil)

	armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2"), arm.Named("arm1_other"), arm.Named("arm2_other"), arm.Named("arm1_other1"), arm.Named("arm2_other1")}
	gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2"), gripper.Named("gripper1_other"), gripper.Named("gripper2_other"), gripper.Named("gripper1_other1"), gripper.Named("gripper2_other1")}
	cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2"), camera.Named("camera1_other"), camera.Named("camera2_other"), camera.Named("camera1_other1"), camera.Named("camera2_other1")}
	servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2"), servo.Named("servo1_other"), servo.Named("servo2_other"), servo.Named("servo1_other1"), servo.Named("servo2_other1")}
	motorNames := []resource.Name{
		motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"), motor.Named("motor1_r2"),
		motor.Named("motor2_r2"), motor.Named("motor1_other"), motor.Named("motor2_other"),
		motor.Named("motor1_other"), motor.Named("motor2_other"), motor.Named("motor1_other1"), motor.Named("motor2_other1"),
	}

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2", "other1"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2", "base1_other", "base2_other", "base1_other1", "base2_other1"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2", "board1_other", "board2_other", "board1_other1", "board2_other1"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2", "sensor1_other", "sensor2_other", "forcematrix_other", "sensor1_other1", "sensor2_other1", "forcematrix_other1"))
	test.That(t, utils.NewStringSet(parts.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(parts.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(parts.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2", "inputController1_other", "inputController2_other", "inputController1_other1", "inputController2_other1"))
	test.That(t, utils.NewStringSet(parts.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2", "func1_other", "func2_other", "func1_other1", "func2_other1"))
	test.That(t, coretestutils.NewResourceNameSet(parts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	emptyParts = newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	sameParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	result, err = parts.MergeAdd(sameParts)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2", "other1"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2", "base1_other", "base2_other", "base1_other1", "base2_other1"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2", "board1_other", "board2_other", "board1_other1", "board2_other1"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2", "sensor1_other", "sensor2_other", "forcematrix_other", "sensor1_other1", "sensor2_other1", "forcematrix_other1"))
	test.That(t, utils.NewStringSet(parts.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(parts.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(parts.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2", "inputController1_other", "inputController2_other", "inputController1_other1", "inputController2_other1"))
	test.That(t, utils.NewStringSet(parts.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2", "func1_other", "func2_other", "func1_other1", "func2_other1"))
	test.That(t, coretestutils.NewResourceNameSet(parts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	emptyParts = newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	test.That(t, utils.NewStringSet(emptyParts.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ArmNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.GripperNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.CameraNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.LidarNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ServoNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.MotorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.InputControllerNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.FunctionNames()...), test.ShouldBeEmpty)
	test.That(t, emptyParts.ResourceNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	err = result.Process(parts)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected")
}

func TestPartsMergeModify(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkSame := func(toCheck *robotParts) {
		t.Helper()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
		motorNames := []resource.Name{
			motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"), motor.Named("motor1_r2"),
			motor.Named("motor2_r2"),
		}

		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
		test.That(t, utils.NewStringSet(toCheck.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
		test.That(t, utils.NewStringSet(toCheck.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
		test.That(t, utils.NewStringSet(toCheck.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
		test.That(t, utils.NewStringSet(toCheck.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
		test.That(t, coretestutils.NewResourceNameSet(parts.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
			armNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
		)...))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		board1, ok := toCheck.BoardByName("board1")
		test.That(t, ok, test.ShouldBeTrue)
		board2r1, ok := toCheck.BoardByName("board2_r1")
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, utils.NewStringSet(board1.AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog1", "analog2"))
		test.That(t, utils.NewStringSet(board1.DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital1", "digital2"))
		test.That(t, utils.NewStringSet(board2r1.AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog1", "analog2"))
		test.That(t, utils.NewStringSet(board2r1.DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital1", "digital2"))
	}
	result, err := parts.MergeModify(context.Background(), newRobotParts(logger), &config.Diff{})
	test.That(t, err, test.ShouldBeNil)
	checkSame(parts)

	emptyParts := newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	test.That(t, utils.NewStringSet(emptyParts.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ArmNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.GripperNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.CameraNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.LidarNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.ServoNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.MotorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.InputControllerNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.FunctionNames()...), test.ShouldBeEmpty)
	test.That(t, emptyParts.ResourceNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldBeEmpty)

	test.That(t, result.Process(parts), test.ShouldBeNil)

	replacementParts := newRobotParts(logger)
	robotForRemote := &localRobot{parts: newRobotParts(logger), logger: logger}
	fakeBoardRemote, err := fake.NewBoard(context.Background(), config.Component{
		Name: "board2",
		ConvertedAttributes: &board.Config{
			Analogs: []board.AnalogConfig{
				{Name: "analog2"},
			},
			DigitalInterrupts: []board.DigitalInterruptConfig{
				{Name: "digital2"},
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	robotForRemote.parts.AddBoard(fakeBoardRemote, config.Component{Name: "board2_r1"})
	robotForRemote.parts.AddLidar(&inject.Lidar{}, config.Component{Name: "lidar2_r1"})
	robotForRemote.parts.AddBase(&inject.Base{}, config.Component{Name: "base2_r1"})
	robotForRemote.parts.AddSensor(&inject.Compass{}, config.Component{Name: "sensor2_r1"})
	robotForRemote.parts.AddInputController(&inject.InputController{}, config.Component{Name: "inputController2_r1"})
	robotForRemote.parts.addFunction("func2_r1")

	cfg := config.Component{Type: config.ComponentTypeArm, Name: "arm2_r1"}
	rName := cfg.ResourceName()
	robotForRemote.parts.addResource(rName, &inject.Arm{})

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

	remote1Replacemenet := newRemoteRobot(robotForRemote, config.Remote{Name: "remote1"})
	replacementParts.addRemote(remote1Replacemenet, config.Remote{Name: "remote1"})

	fakeBoard, err := fake.NewBoard(context.Background(), config.Component{
		Name: "board1",
		ConvertedAttributes: &board.Config{
			Analogs: []board.AnalogConfig{
				{Name: "analog2"},
			},
			DigitalInterrupts: []board.DigitalInterruptConfig{
				{Name: "digital2"},
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	replacementParts.AddBoard(fakeBoard, config.Component{Name: "board1"})
	injectLidar := &inject.Lidar{}
	replacementParts.AddLidar(injectLidar, config.Component{Name: "lidar1"})
	injectBase := &inject.Base{}
	replacementParts.AddBase(injectBase, config.Component{Name: "base1"})
	injectCompass := &inject.Compass{}
	replacementParts.AddSensor(injectCompass, config.Component{Name: "sensor1"})
	injectInputController := &inject.InputController{}
	replacementParts.AddInputController(injectInputController, config.Component{Name: "inputController1"})
	cfg = config.Component{Type: config.ComponentTypeArm, Name: "arm1"}
	rName = cfg.ResourceName()
	replacementParts.addResource(rName, &inject.Arm{})
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
	fp1 := &fakeProcess{id: "1"}
	_, err = replacementParts.processManager.AddProcess(context.Background(), fp1, false)
	test.That(t, err, test.ShouldBeNil)
}

func TestPartsMergeRemove(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkSame := func(toCheck *robotParts) {
		t.Helper()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
		gripperNames := []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
		cameraNames := []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
		servoNames := []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
		motorNames := []resource.Name{
			motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"), motor.Named("motor1_r2"),
			motor.Named("motor2_r2"),
		}

		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "forcematrix", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
		test.That(t, utils.NewStringSet(toCheck.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
		test.That(t, utils.NewStringSet(toCheck.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
		test.That(t, utils.NewStringSet(toCheck.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
		test.That(t, utils.NewStringSet(toCheck.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
		test.That(t, coretestutils.NewResourceNameSet(toCheck.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
			armNames,
			gripperNames,
			cameraNames,
			servoNames,
			motorNames,
		)...))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
	}

	parts.MergeRemove(newRobotParts(logger))
	checkSame(parts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_other1"), config.Remote{}), config.Remote{Name: "other1"})
	parts.MergeRemove(otherParts)
	checkSame(parts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	sameParts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
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
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldBeEmpty)
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
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	checkEmpty := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldBeEmpty)
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

	filtered, err := parts.FilterFromConfig(&config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(&config.Config{
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
				Name: "what4",
				Type: config.ComponentTypeLidar,
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

	filtered, err = parts.FilterFromConfig(&config.Config{
		Components: []config.Component{
			{
				Name: "what1",
				Type: "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(&config.Config{
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
				Name: "lidar2",
				Type: config.ComponentTypeLidar,
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
	gripperNames := []resource.Name{gripper.Named("gripper2")}
	cameraNames := []resource.Name{camera.Named("camera2")}
	servoNames := []resource.Name{servo.Named("servo2")}
	motorNames := []resource.Name{motor.Named("motor2")}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2"))
	test.That(t, utils.NewStringSet(filtered.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(filtered.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(filtered.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController2"))
	test.That(t, utils.NewStringSet(filtered.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func2"))
	test.That(t, coretestutils.NewResourceNameSet(filtered.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(filtered.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("2"))

	filtered, err = parts.FilterFromConfig(&config.Config{
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
				Name: "lidar2",
				Type: config.ComponentTypeLidar,
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
	gripperNames = []resource.Name{gripper.Named("gripper2"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
	cameraNames = []resource.Name{camera.Named("camera2"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
	servoNames = []resource.Name{servo.Named("servo2"), servo.Named("servo1_r2"), servo.Named("servo2_r2")}
	motorNames = []resource.Name{motor.Named("motor2"), motor.Named("motor1_r2"), motor.Named("motor2_r2")}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote2"))
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
	test.That(t, utils.NewStringSet(filtered.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(filtered.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(filtered.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController2", "inputController1_r2", "inputController2_r2"))
	test.That(t, utils.NewStringSet(filtered.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func2", "func1_r2", "func2_r2"))
	test.That(t, coretestutils.NewResourceNameSet(filtered.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(filtered.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("2"))

	filtered, err = parts.FilterFromConfig(&config.Config{
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
				Name: "lidar1",
				Type: config.ComponentTypeLidar,
			},
			{
				Name: "lidar2",
				Type: config.ComponentTypeLidar,
			},
			{
				Name: "lidar3",
				Type: config.ComponentTypeLidar,
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

	armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("arm1_r1"), arm.Named("arm2_r1"), arm.Named("arm1_r2"), arm.Named("arm2_r2")}
	gripperNames = []resource.Name{gripper.Named("gripper1"), gripper.Named("gripper2"), gripper.Named("gripper1_r1"), gripper.Named("gripper2_r1"), gripper.Named("gripper1_r2"), gripper.Named("gripper2_r2")}
	cameraNames = []resource.Name{camera.Named("camera1"), camera.Named("camera2"), camera.Named("camera1_r1"), camera.Named("camera2_r1"), camera.Named("camera1_r2"), camera.Named("camera2_r2")}
	servoNames = []resource.Name{servo.Named("servo1"), servo.Named("servo2"), servo.Named("servo1_r1"), servo.Named("servo1_r2"), servo.Named("servo2_r1"), servo.Named("servo2_r2")}
	motorNames = []resource.Name{
		motor.Named("motor1"), motor.Named("motor2"), motor.Named("motor1_r1"), motor.Named("motor2_r1"), motor.Named("motor1_r2"),
		motor.Named("motor2_r2"),
	}

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(armNames...)...))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(gripperNames...)...))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(cameraNames...)...))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "forcematrix_r1", "sensor1_r2", "sensor2_r2", "forcematrix_r2"))
	test.That(t, utils.NewStringSet(filtered.ServoNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(servoNames...)...))
	test.That(t, utils.NewStringSet(filtered.MotorNames()...), test.ShouldResemble, utils.NewStringSet(coretestutils.ExtractNames(motorNames...)...))
	test.That(t, utils.NewStringSet(filtered.InputControllerNames()...), test.ShouldResemble, utils.NewStringSet("inputController1", "inputController2", "inputController1_r1", "inputController2_r1", "inputController1_r2", "inputController2_r2"))
	test.That(t, utils.NewStringSet(filtered.FunctionNames()...), test.ShouldResemble, utils.NewStringSet("func1", "func2", "func1_r1", "func2_r1", "func1_r2", "func2_r2"))
	test.That(t, coretestutils.NewResourceNameSet(filtered.ResourceNames()...), test.ShouldResemble, coretestutils.NewResourceNameSet(coretestutils.ConcatResourceNames(
		armNames,
		gripperNames,
		cameraNames,
		servoNames,
		motorNames,
	)...))
	test.That(t, utils.NewStringSet(filtered.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
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
