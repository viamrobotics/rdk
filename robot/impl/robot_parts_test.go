package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/rexec"
	"go.viam.com/core/robot"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestPartsForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)

	test.That(t, parts.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2"))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2"))

	arm1 := parts.ArmByName("arm1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	test.That(t, parts.ArmByName("arm1_what"), test.ShouldBeNil)
	base1 := parts.BaseByName("base1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	test.That(t, parts.BaseByName("base1_what"), test.ShouldBeNil)
	gripper1 := parts.GripperByName("gripper1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	test.That(t, parts.GripperByName("gripper1_what"), test.ShouldBeNil)
	camera1 := parts.CameraByName("camera1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	test.That(t, parts.CameraByName("camera1_what"), test.ShouldBeNil)
	lidar1 := parts.LidarByName("lidar1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	test.That(t, parts.LidarByName("lidar1_what"), test.ShouldBeNil)
	board1 := parts.BoardByName("board1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	test.That(t, parts.BoardByName("board1_what"), test.ShouldBeNil)
	sensor1 := parts.SensorByName("sensor1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	test.That(t, parts.SensorByName("sensor1_what"), test.ShouldBeNil)
}

func TestPartsMergeNamesWithRemotes(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r1"), config.Remote{}), config.Remote{Name: "remote1"})
	parts.addRemote(newRemoteRobot(setupInjectRobotWithSuffx(logger, "_r2"), config.Remote{}), config.Remote{Name: "remote2"})

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))

	arm1 := parts.ArmByName("arm1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	arm1 = parts.ArmByName("arm1_r1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1_r1")
	arm1 = parts.ArmByName("arm1_r2")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1_r2")
	test.That(t, parts.ArmByName("arm1_what"), test.ShouldBeNil)

	base1 := parts.BaseByName("base1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1 = parts.BaseByName("base1_r1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1 = parts.BaseByName("base1_r2")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
	test.That(t, parts.BaseByName("base1_what"), test.ShouldBeNil)

	gripper1 := parts.GripperByName("gripper1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	gripper1 = parts.GripperByName("gripper1_r1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r1")
	gripper1 = parts.GripperByName("gripper1_r2")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r2")
	test.That(t, parts.GripperByName("gripper1_what"), test.ShouldBeNil)

	camera1 := parts.CameraByName("camera1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	camera1 = parts.CameraByName("camera1_r1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1_r1")
	camera1 = parts.CameraByName("camera1_r2")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1_r2")
	test.That(t, parts.CameraByName("camera1_what"), test.ShouldBeNil)

	lidar1 := parts.LidarByName("lidar1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1 = parts.LidarByName("lidar1_r1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1 = parts.LidarByName("lidar1_r2")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	test.That(t, parts.LidarByName("lidar1_what"), test.ShouldBeNil)

	board1 := parts.BoardByName("board1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	board1 = parts.BoardByName("board1_r1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r1")
	board1 = parts.BoardByName("board1_r2")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r2")
	test.That(t, parts.BoardByName("board1_what"), test.ShouldBeNil)

	sensor1 := parts.SensorByName("sensor1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1 = parts.SensorByName("sensor1_r1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1 = parts.SensorByName("sensor1_r2")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
	test.That(t, parts.SensorByName("sensor1_what"), test.ShouldBeNil)
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
	delete(parts.arms, "arm1")
	parts.arms = nil
	delete(parts.grippers, "gripper1")
	parts.grippers = nil
	delete(parts.cameras, "camera1")
	parts.cameras = nil
	delete(parts.lidars, "lidar1")
	parts.lidars = nil
	delete(parts.bases, "base1")
	parts.bases = nil
	delete(parts.boards, "board1")
	parts.boards = nil
	delete(parts.sensors, "sensor1")
	parts.sensors = nil
	_, ok := parts.processManager.RemoveProcessByID("1")
	test.That(t, ok, test.ShouldBeTrue)
	parts.processManager.Stop()

	test.That(t, utils.NewStringSet(newParts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(newParts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(newParts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(newParts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(newParts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(newParts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(newParts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(newParts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
	test.That(t, utils.NewStringSet(newParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	arm1 := newParts.ArmByName("arm1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	arm1 = newParts.ArmByName("arm1_r1")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1_r1")
	arm1 = newParts.ArmByName("arm1_r2")
	test.That(t, arm1.(*proxyArm).actual.(*fake.Arm).Name, test.ShouldEqual, "arm1_r2")
	test.That(t, newParts.ArmByName("arm1_what"), test.ShouldBeNil)

	base1 := newParts.BaseByName("base1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1 = newParts.BaseByName("base1_r1")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1 = newParts.BaseByName("base1_r2")
	test.That(t, base1.(*proxyBase).actual.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
	test.That(t, newParts.BaseByName("base1_what"), test.ShouldBeNil)

	gripper1 := newParts.GripperByName("gripper1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	gripper1 = newParts.GripperByName("gripper1_r1")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r1")
	gripper1 = newParts.GripperByName("gripper1_r2")
	test.That(t, gripper1.(*proxyGripper).actual.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r2")
	test.That(t, newParts.GripperByName("gripper1_what"), test.ShouldBeNil)

	camera1 := newParts.CameraByName("camera1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	camera1 = newParts.CameraByName("camera1_r1")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1_r1")
	camera1 = newParts.CameraByName("camera1_r2")
	test.That(t, camera1.(*proxyCamera).actual.(*fake.Camera).Name, test.ShouldEqual, "camera1_r2")
	test.That(t, newParts.CameraByName("camera1_what"), test.ShouldBeNil)

	lidar1 := newParts.LidarByName("lidar1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1 = newParts.LidarByName("lidar1_r1")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1 = newParts.LidarByName("lidar1_r2")
	test.That(t, lidar1.(*proxyLidar).actual.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	test.That(t, newParts.LidarByName("lidar1_what"), test.ShouldBeNil)

	board1 := newParts.BoardByName("board1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	board1 = newParts.BoardByName("board1_r1")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r1")
	board1 = newParts.BoardByName("board1_r2")
	test.That(t, board1.(*proxyBoard).actual.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r2")
	test.That(t, newParts.BoardByName("board1_what"), test.ShouldBeNil)

	sensor1 := newParts.SensorByName("sensor1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1 = newParts.SensorByName("sensor1_r1")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1 = newParts.SensorByName("sensor1_r2")
	test.That(t, sensor1.(*proxyCompass).actual.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
	test.That(t, newParts.SensorByName("sensor1_what"), test.ShouldBeNil)

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
	injectBoard.MotorNamesFunc = func() []string {
		return []string{"motor1"}
	}
	injectBoard.ServoNamesFunc = func() []string {
		return []string{"servo1"}
	}
	injectBoard.AnalogReaderNamesFunc = func() []string {
		return []string{"analog1"}
	}
	injectBoard.DigitalInterruptNamesFunc = func() []string {
		return []string{"digital1"}
	}
	injectBoard.MotorFunc = func(name string) board.Motor {
		return &inject.Motor{}
	}
	injectBoard.ServoFunc = func(name string) board.Servo {
		return &inject.Servo{}
	}
	injectBoard.AnalogReaderFunc = func(name string) board.AnalogReader {
		return &board.FakeAnalog{}
	}
	injectBoard.DigitalInterruptFunc = func(name string) board.DigitalInterrupt {
		return &board.BasicDigitalInterrupt{}
	}
	parts.AddBoard(injectBoard, board.Config{Name: "board1"})
	test.That(t, parts.BoardByName("board1").(*proxyBoard).actual, test.ShouldEqual, injectBoard)
	parts.AddBoard(parts.BoardByName("board1"), board.Config{Name: "board1"})
	test.That(t, parts.BoardByName("board1").(*proxyBoard).actual, test.ShouldEqual, injectBoard)

	injectArm := &inject.Arm{}
	parts.AddArm(injectArm, config.Component{Name: "arm1"})
	test.That(t, parts.ArmByName("arm1").(*proxyArm).actual, test.ShouldEqual, injectArm)
	parts.AddArm(parts.ArmByName("arm1"), config.Component{Name: "arm1"})
	test.That(t, parts.ArmByName("arm1").(*proxyArm).actual, test.ShouldEqual, injectArm)

	injectGripper := &inject.Gripper{}
	parts.AddGripper(injectGripper, config.Component{Name: "gripper1"})
	test.That(t, parts.GripperByName("gripper1").(*proxyGripper).actual, test.ShouldEqual, injectGripper)
	parts.AddGripper(parts.GripperByName("gripper1"), config.Component{Name: "gripper1"})
	test.That(t, parts.GripperByName("gripper1").(*proxyGripper).actual, test.ShouldEqual, injectGripper)

	injectCamera := &inject.Camera{}
	parts.AddCamera(injectCamera, config.Component{Name: "camera1"})
	test.That(t, parts.CameraByName("camera1").(*proxyCamera).actual, test.ShouldEqual, injectCamera)
	parts.AddCamera(parts.CameraByName("camera1"), config.Component{Name: "camera1"})
	test.That(t, parts.CameraByName("camera1").(*proxyCamera).actual, test.ShouldEqual, injectCamera)

	injectLidar := &inject.Lidar{}
	parts.AddLidar(injectLidar, config.Component{Name: "lidar1"})
	test.That(t, parts.LidarByName("lidar1").(*proxyLidar).actual, test.ShouldEqual, injectLidar)
	parts.AddLidar(parts.LidarByName("lidar1"), config.Component{Name: "lidar1"})
	test.That(t, parts.LidarByName("lidar1").(*proxyLidar).actual, test.ShouldEqual, injectLidar)

	injectBase := &inject.Base{}
	parts.AddBase(injectBase, config.Component{Name: "base1"})
	test.That(t, parts.BaseByName("base1").(*proxyBase).actual, test.ShouldEqual, injectBase)
	parts.AddBase(parts.BaseByName("base1"), config.Component{Name: "base1"})
	test.That(t, parts.BaseByName("base1").(*proxyBase).actual, test.ShouldEqual, injectBase)

	injectSensor := &inject.Sensor{}
	parts.AddSensor(injectSensor, config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxySensor).actual, test.ShouldEqual, injectSensor)
	parts.AddSensor(parts.SensorByName("sensor1"), config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxySensor).actual, test.ShouldEqual, injectSensor)

	injectCompass := &inject.Compass{}
	parts.AddSensor(injectCompass, config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxyCompass).actual, test.ShouldEqual, injectCompass)
	parts.AddSensor(parts.SensorByName("sensor1"), config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxyCompass).actual, test.ShouldEqual, injectCompass)

	injectRelativeCompass := &inject.RelativeCompass{}
	parts.AddSensor(injectRelativeCompass, config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxyRelativeCompass).actual, test.ShouldEqual, injectRelativeCompass)
	parts.AddSensor(parts.SensorByName("sensor1"), config.Component{Name: "sensor1"})
	test.That(t, parts.SensorByName("sensor1").(*proxyRelativeCompass).actual, test.ShouldEqual, injectRelativeCompass)

	dummyProv := &dummyProvider{}
	parts.AddProvider(dummyProv, config.Component{Name: "provider1"})
	test.That(t, parts.ProviderByName("provider1").(*proxyProvider).actual, test.ShouldEqual, dummyProv)
	parts.AddProvider(parts.ProviderByName("provider1"), config.Component{Name: "provider1"})
	test.That(t, parts.ProviderByName("provider1").(*proxyProvider).actual, test.ShouldEqual, dummyProv)
}

type dummyProvider struct {
}

func (dp *dummyProvider) Ready(r robot.Robot) error {
	return nil
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
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}
	checkSame := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
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

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2", "other1"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2", "arm1_other", "arm2_other", "arm1_other1", "arm2_other1"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2", "gripper1_other", "gripper2_other", "gripper1_other1", "gripper2_other1"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2", "camera1_other", "camera2_other", "camera1_other1", "camera2_other1"))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2", "base1_other", "base2_other", "base1_other1", "base2_other1"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2", "board1_other", "board2_other", "board1_other1", "board2_other1"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2", "sensor1_other", "sensor2_other", "sensor1_other1", "sensor2_other1"))
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
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2", "arm1_other", "arm2_other", "arm1_other1", "arm2_other1"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2", "gripper1_other", "gripper2_other", "gripper1_other1", "gripper2_other1"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2", "camera1_other", "camera2_other", "camera1_other1", "camera2_other1"))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2", "base1_other", "base2_other", "base1_other1", "base2_other1"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2", "board1_other", "board2_other", "board1_other1", "board2_other1"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2", "sensor1_other", "sensor2_other", "sensor1_other1", "sensor2_other1"))
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
	parts.AddProvider(&dummyProvider{}, config.Component{Name: "provider1"})

	checkSame := func(toCheck *robotParts) {
		t.Helper()
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board1").MotorNames()...), test.ShouldResemble, utils.NewStringSet("motor1", "motor2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board1").ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo1", "servo2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board1").AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog1", "analog2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board1").DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital1", "digital2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board2_r1").MotorNames()...), test.ShouldResemble, utils.NewStringSet("motor1", "motor2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board2_r1").ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo1", "servo2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board2_r1").AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog1", "analog2"))
		test.That(t, utils.NewStringSet(toCheck.BoardByName("board2_r1").DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital1", "digital2"))
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
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldBeEmpty)

	test.That(t, result.Process(parts), test.ShouldBeNil)

	replacementParts := newRobotParts(logger)
	robotForRemote := &mutableRobot{parts: newRobotParts(logger), logger: logger}
	fakeBoardRemote, err := board.NewFakeBoard(context.Background(), board.Config{
		Name: "board2",
		Motors: []board.MotorConfig{
			{Name: "motor2"},
		},
		Servos: []board.ServoConfig{
			{Name: "servo2"},
		},
		Analogs: []board.AnalogConfig{
			{Name: "analog2"},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "digital2"},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	robotForRemote.parts.AddBoard(fakeBoardRemote, board.Config{Name: "board2_r1"})
	robotForRemote.parts.AddArm(&inject.Arm{}, config.Component{Name: "arm2_r1"})
	robotForRemote.parts.AddGripper(&inject.Gripper{}, config.Component{Name: "gripper2_r1"})
	robotForRemote.parts.AddLidar(&inject.Lidar{}, config.Component{Name: "lidar2_r1"})
	robotForRemote.parts.AddCamera(&inject.Camera{}, config.Component{Name: "camera2_r1"})
	robotForRemote.parts.AddBase(&inject.Base{}, config.Component{Name: "base2_r1"})
	robotForRemote.parts.AddSensor(&inject.Compass{}, config.Component{Name: "sensor2_r1"})
	robotForRemote.parts.AddProvider(&dummyProvider{}, config.Component{Name: "provider1_r1"})

	remote1Replacemenet := newRemoteRobot(robotForRemote, config.Remote{Name: "remote1"})
	replacementParts.addRemote(remote1Replacemenet, config.Remote{Name: "remote1"})

	fakeBoard, err := board.NewFakeBoard(context.Background(), board.Config{
		Name: "board1",
		Motors: []board.MotorConfig{
			{Name: "motor2"},
		},
		Servos: []board.ServoConfig{
			{Name: "servo2"},
		},
		Analogs: []board.AnalogConfig{
			{Name: "analog2"},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "digital2"},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	replacementParts.AddBoard(fakeBoard, board.Config{Name: "board1"})
	injectArm := &inject.Arm{}
	replacementParts.AddArm(injectArm, config.Component{Name: "arm1"})
	injectGripper := &inject.Gripper{}
	replacementParts.AddGripper(injectGripper, config.Component{Name: "gripper1"})
	injectLidar := &inject.Lidar{}
	replacementParts.AddLidar(injectLidar, config.Component{Name: "lidar1"})
	injectCamera := &inject.Camera{}
	replacementParts.AddCamera(injectCamera, config.Component{Name: "camera1"})
	injectBase := &inject.Base{}
	replacementParts.AddBase(injectBase, config.Component{Name: "base1"})
	injectCompass := &inject.Compass{}
	replacementParts.AddSensor(injectCompass, config.Component{Name: "sensor1"})
	injectProvider := &dummyProvider{}
	replacementParts.AddProvider(injectProvider, config.Component{Name: "provider1"})
	fp1 := &fakeProcess{id: "1"}
	_, err = replacementParts.processManager.AddProcess(context.Background(), fp1, false)
	test.That(t, err, test.ShouldBeNil)

	remote1Before := parts.RemoteByName("remote1")
	boardBefore := parts.BoardByName("board1")
	armBefore := parts.ArmByName("arm1")
	gripperBefore := parts.GripperByName("gripper1")
	lidarBefore := parts.LidarByName("lidar1")
	cameraBefore := parts.CameraByName("camera1")
	baseBefore := parts.BaseByName("base1")
	compassBefore := parts.SensorByName("sensor1")
	providerBefore := parts.ProviderByName("provider1")
	motorBefore := parts.BoardByName("board1").Motor("motor2")
	servoBefore := parts.BoardByName("board1").Servo("servo2")
	analogBefore := parts.BoardByName("board1").AnalogReader("analog2")
	digitalBefore := parts.BoardByName("board1").DigitalInterrupt("digital2")
	motorRemoteBefore := parts.BoardByName("board2_r1").Motor("motor2")
	servoRemoteBefore := parts.BoardByName("board2_r1").Servo("servo2")
	analogRemoteBefore := parts.BoardByName("board2_r1").AnalogReader("analog2")
	digitalRemoteBefore := parts.BoardByName("board2_r1").DigitalInterrupt("digital2")

	result, err = parts.MergeModify(context.Background(), replacementParts, &config.Diff{
		Modified: &config.ModifiedConfigDiff{
			Boards: map[string]board.ConfigDiff{
				"board1": {
					Left: &board.Config{
						Model: "one",
					},
					Right: &board.Config{
						Model: "one",
					},
					Added: &board.Config{
						Analogs: []board.AnalogConfig{
							{Name: "analog2"},
						},
						DigitalInterrupts: []board.DigitalInterruptConfig{
							{Name: "digital2"},
						},
					},
					Removed: &board.Config{
						Motors: []board.MotorConfig{
							{Name: "motor1"},
						},
						Servos: []board.ServoConfig{
							{Name: "servo1"},
						},
						Analogs: []board.AnalogConfig{
							{Name: "analog1"},
						},
						DigitalInterrupts: []board.DigitalInterruptConfig{
							{Name: "digital1"},
						},
					},
					Modified: &board.Config{
						Motors: []board.MotorConfig{
							{Name: "motor2"},
						},
						Servos: []board.ServoConfig{
							{Name: "servo2"},
						},
					},
				},
				"board2": {
					Left: &board.Config{
						Model: "two",
					},
					Right: &board.Config{
						Model: "two",
					},
					Added:    &board.Config{},
					Removed:  &board.Config{},
					Modified: &board.Config{},
				},
			},
		},
	})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(parts.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board1").MotorNames()...), test.ShouldResemble, utils.NewStringSet("motor2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board1").ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board1").AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board1").DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board2_r1").MotorNames()...), test.ShouldResemble, utils.NewStringSet("motor2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board2_r1").ServoNames()...), test.ShouldResemble, utils.NewStringSet("servo2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board2_r1").AnalogReaderNames()...), test.ShouldResemble, utils.NewStringSet("analog2"))
	test.That(t, utils.NewStringSet(parts.BoardByName("board2_r1").DigitalInterruptNames()...), test.ShouldResemble, utils.NewStringSet("digital2"))

	test.That(t, parts.RemoteByName("remote1"), test.ShouldEqual, remote1Before)
	test.That(t, parts.ArmByName("arm1").(*proxyArm).actual, test.ShouldEqual, injectArm)
	test.That(t, parts.ArmByName("arm1").(*proxyArm), test.ShouldEqual, armBefore)
	test.That(t, parts.GripperByName("gripper1").(*proxyGripper).actual, test.ShouldEqual, injectGripper)
	test.That(t, parts.GripperByName("gripper1").(*proxyGripper), test.ShouldEqual, gripperBefore)
	test.That(t, parts.CameraByName("camera1").(*proxyCamera).actual, test.ShouldEqual, injectCamera)
	test.That(t, parts.CameraByName("camera1").(*proxyCamera), test.ShouldEqual, cameraBefore)
	test.That(t, parts.LidarByName("lidar1").(*proxyLidar).actual, test.ShouldEqual, injectLidar)
	test.That(t, parts.LidarByName("lidar1").(*proxyLidar), test.ShouldEqual, lidarBefore)
	test.That(t, parts.BaseByName("base1").(*proxyBase).actual, test.ShouldEqual, injectBase)
	test.That(t, parts.BaseByName("base1").(*proxyBase), test.ShouldEqual, baseBefore)
	test.That(t, parts.BoardByName("board1").(*proxyBoard).actual, test.ShouldEqual, fakeBoard)
	test.That(t, parts.BoardByName("board1").(*proxyBoard), test.ShouldEqual, boardBefore)
	test.That(t, parts.SensorByName("sensor1").(*proxyCompass).actual, test.ShouldEqual, injectCompass)
	test.That(t, parts.SensorByName("sensor1").(*proxyCompass), test.ShouldEqual, compassBefore)
	test.That(t, parts.ProviderByName("provider1").(*proxyProvider).actual, test.ShouldEqual, injectProvider)
	test.That(t, parts.ProviderByName("provider1").(*proxyProvider), test.ShouldEqual, providerBefore)
	test.That(t, parts.BoardByName("board1").Motor("motor2").(*proxyBoardMotor).actual, test.ShouldEqual, fakeBoard.Motor("motor2"))
	test.That(t, parts.BoardByName("board1").Motor("motor2").(*proxyBoardMotor), test.ShouldEqual, motorBefore)
	test.That(t, parts.BoardByName("board1").Servo("servo2").(*proxyBoardServo).actual, test.ShouldEqual, fakeBoard.Servo("servo2"))
	test.That(t, parts.BoardByName("board1").Servo("servo2").(*proxyBoardServo), test.ShouldEqual, servoBefore)
	test.That(t, parts.BoardByName("board1").AnalogReader("analog2").(*proxyBoardAnalogReader).actual, test.ShouldEqual, fakeBoard.AnalogReader("analog2"))
	test.That(t, parts.BoardByName("board1").AnalogReader("analog2").(*proxyBoardAnalogReader), test.ShouldNotEqual, analogBefore)
	test.That(t, parts.BoardByName("board1").AnalogReader("analog2").(*proxyBoardAnalogReader), test.ShouldEqual, replacementParts.BoardByName("board1").AnalogReader("analog2"))
	test.That(t, parts.BoardByName("board1").DigitalInterrupt("digital2").(*proxyBoardDigitalInterrupt).actual, test.ShouldEqual, fakeBoard.DigitalInterrupt("digital2"))
	test.That(t, parts.BoardByName("board1").DigitalInterrupt("digital2").(*proxyBoardDigitalInterrupt), test.ShouldNotEqual, digitalBefore)
	test.That(t, parts.BoardByName("board1").DigitalInterrupt("digital2").(*proxyBoardDigitalInterrupt), test.ShouldEqual, replacementParts.BoardByName("board1").DigitalInterrupt("digital2"))
	test.That(t, parts.BoardByName("board2_r1").Motor("motor2").(*proxyBoardMotor).actual, test.ShouldEqual, fakeBoardRemote.Motor("motor2"))
	test.That(t, parts.BoardByName("board2_r1").Motor("motor2").(*proxyBoardMotor), test.ShouldEqual, motorRemoteBefore)
	test.That(t, parts.BoardByName("board2_r1").Servo("servo2").(*proxyBoardServo).actual, test.ShouldEqual, fakeBoardRemote.Servo("servo2"))
	test.That(t, parts.BoardByName("board2_r1").Servo("servo2").(*proxyBoardServo), test.ShouldEqual, servoRemoteBefore)
	test.That(t, parts.BoardByName("board2_r1").AnalogReader("analog2").(*proxyBoardAnalogReader).actual, test.ShouldEqual, fakeBoardRemote.AnalogReader("analog2"))
	test.That(t, parts.BoardByName("board2_r1").AnalogReader("analog2").(*proxyBoardAnalogReader), test.ShouldEqual, analogRemoteBefore)
	test.That(t, parts.BoardByName("board2_r1").DigitalInterrupt("digital2").(*proxyBoardDigitalInterrupt).actual, test.ShouldEqual, fakeBoardRemote.DigitalInterrupt("digital2"))
	test.That(t, parts.BoardByName("board2_r1").DigitalInterrupt("digital2").(*proxyBoardDigitalInterrupt), test.ShouldEqual, digitalRemoteBefore)

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
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1"))

	err = result.Process(parts)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected")
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
		test.That(t, utils.NewStringSet(toCheck.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
		test.That(t, utils.NewStringSet(toCheck.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
		test.That(t, utils.NewStringSet(toCheck.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
		test.That(t, utils.NewStringSet(toCheck.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
		test.That(t, utils.NewStringSet(toCheck.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
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
		},
		Boards: []board.Config{
			{
				Name: "what",
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "what",
				Name: "echo",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	_, err = parts.FilterFromConfig(&config.Config{
		Components: []config.Component{
			{
				Name: "what1",
				Type: "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")
	test.That(t, err.Error(), test.ShouldContainSubstring, "something")

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
		},
		Boards: []board.Config{
			{
				Name: "board2",
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm2"))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper2"))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera2"))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2"))
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
		},
		Boards: []board.Config{
			{
				Name: "board2",
			},
		},
		Processes: []rexec.ProcessConfig{
			{
				ID:   "2",
				Name: "echo", // does not matter
			},
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote2"))
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm2", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper2", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera2", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar2", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2", "sensor1_r2", "sensor2_r2"))
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
		},
		Boards: []board.Config{
			{
				Name: "board1",
			},
			{
				Name: "board2",
			},
			{
				Name: "board3",
			},
		},
		Processes: []rexec.ProcessConfig{
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

	test.That(t, utils.NewStringSet(filtered.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(filtered.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(filtered.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(filtered.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(filtered.LidarNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
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
