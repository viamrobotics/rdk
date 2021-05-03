package robot

import (
	"context"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rexec"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestPartsForRemoteRobot(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)

	test.That(t, parts.RemoteNames(), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2"))
	test.That(t, utils.NewStringSet(parts.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2"))

	arm1 := parts.ArmByName("arm1")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	test.That(t, parts.ArmByName("arm1_what"), test.ShouldBeNil)
	base1 := parts.BaseByName("base1")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1")
	test.That(t, parts.BaseByName("base1_what"), test.ShouldBeNil)
	gripper1 := parts.GripperByName("gripper1")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	test.That(t, parts.GripperByName("gripper1_what"), test.ShouldBeNil)
	camera1 := parts.CameraByName("camera1")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	test.That(t, parts.CameraByName("camera1_what"), test.ShouldBeNil)
	lidar1 := parts.LidarDeviceByName("lidar1")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	test.That(t, parts.LidarDeviceByName("lidar1_what"), test.ShouldBeNil)
	board1 := parts.BoardByName("board1")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	test.That(t, parts.BoardByName("board1_what"), test.ShouldBeNil)
	sensor1 := parts.SensorByName("sensor1")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	test.That(t, parts.SensorByName("sensor1_what"), test.ShouldBeNil)
}

func TestPartsMergeNamesWithRemotes(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(parts.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))

	arm1 := parts.ArmByName("arm1")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	arm1 = parts.ArmByName("arm1_r1")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1_r1")
	arm1 = parts.ArmByName("arm1_r2")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1_r2")
	test.That(t, parts.ArmByName("arm1_what"), test.ShouldBeNil)

	base1 := parts.BaseByName("base1")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1 = parts.BaseByName("base1_r1")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1 = parts.BaseByName("base1_r2")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
	test.That(t, parts.BaseByName("base1_what"), test.ShouldBeNil)

	gripper1 := parts.GripperByName("gripper1")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	gripper1 = parts.GripperByName("gripper1_r1")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r1")
	gripper1 = parts.GripperByName("gripper1_r2")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r2")
	test.That(t, parts.GripperByName("gripper1_what"), test.ShouldBeNil)

	camera1 := parts.CameraByName("camera1")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	camera1 = parts.CameraByName("camera1_r1")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1_r1")
	camera1 = parts.CameraByName("camera1_r2")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1_r2")
	test.That(t, parts.CameraByName("camera1_what"), test.ShouldBeNil)

	lidar1 := parts.LidarDeviceByName("lidar1")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1 = parts.LidarDeviceByName("lidar1_r1")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1 = parts.LidarDeviceByName("lidar1_r2")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	test.That(t, parts.LidarDeviceByName("lidar1_what"), test.ShouldBeNil)

	board1 := parts.BoardByName("board1")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	board1 = parts.BoardByName("board1_r1")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r1")
	board1 = parts.BoardByName("board1_r2")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r2")
	test.That(t, parts.BoardByName("board1_what"), test.ShouldBeNil)

	sensor1 := parts.SensorByName("sensor1")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1 = parts.SensorByName("sensor1_r1")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1 = parts.SensorByName("sensor1_r2")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
	test.That(t, parts.SensorByName("sensor1_what"), test.ShouldBeNil)
}

func TestPartsClone(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
	_, err := parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = parts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	newParts := parts.Clone()

	// remove and delet eparts to prove clone
	delete(parts.remotes, "remote1")
	parts.remotes = nil
	delete(parts.arms, "arm1")
	parts.arms = nil
	delete(parts.grippers, "gripper1")
	parts.grippers = nil
	delete(parts.cameras, "camera1")
	parts.cameras = nil
	delete(parts.lidarDevices, "lidar1")
	parts.lidarDevices = nil
	delete(parts.bases, "base1")
	parts.bases = nil
	delete(parts.boards, "board1")
	parts.boards = nil
	delete(parts.sensors, "sensor1")
	parts.sensors = nil
	test.That(t, parts.processManager.RemoveProcessByID("1"), test.ShouldBeTrue)
	parts.processManager.Stop()

	test.That(t, utils.NewStringSet(newParts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2"))
	test.That(t, utils.NewStringSet(newParts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2"))
	test.That(t, utils.NewStringSet(newParts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2"))
	test.That(t, utils.NewStringSet(newParts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2"))
	test.That(t, utils.NewStringSet(newParts.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(newParts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(newParts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(newParts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
	test.That(t, utils.NewStringSet(newParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	arm1 := newParts.ArmByName("arm1")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1")
	arm1 = newParts.ArmByName("arm1_r1")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1_r1")
	arm1 = newParts.ArmByName("arm1_r2")
	test.That(t, arm1.(*fake.Arm).Name, test.ShouldEqual, "arm1_r2")
	test.That(t, newParts.ArmByName("arm1_what"), test.ShouldBeNil)

	base1 := newParts.BaseByName("base1")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1")
	base1 = newParts.BaseByName("base1_r1")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1_r1")
	base1 = newParts.BaseByName("base1_r2")
	test.That(t, base1.(*fake.Base).Name, test.ShouldEqual, "base1_r2")
	test.That(t, newParts.BaseByName("base1_what"), test.ShouldBeNil)

	gripper1 := newParts.GripperByName("gripper1")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1")
	gripper1 = newParts.GripperByName("gripper1_r1")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r1")
	gripper1 = newParts.GripperByName("gripper1_r2")
	test.That(t, gripper1.(*fake.Gripper).Name, test.ShouldEqual, "gripper1_r2")
	test.That(t, newParts.GripperByName("gripper1_what"), test.ShouldBeNil)

	camera1 := newParts.CameraByName("camera1")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1")
	camera1 = newParts.CameraByName("camera1_r1")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1_r1")
	camera1 = newParts.CameraByName("camera1_r2")
	test.That(t, camera1.(*fake.Camera).Name, test.ShouldEqual, "camera1_r2")
	test.That(t, newParts.CameraByName("camera1_what"), test.ShouldBeNil)

	lidar1 := newParts.LidarDeviceByName("lidar1")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1")
	lidar1 = newParts.LidarDeviceByName("lidar1_r1")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r1")
	lidar1 = newParts.LidarDeviceByName("lidar1_r2")
	test.That(t, lidar1.(*fake.Lidar).Name, test.ShouldEqual, "lidar1_r2")
	test.That(t, newParts.LidarDeviceByName("lidar1_what"), test.ShouldBeNil)

	board1 := newParts.BoardByName("board1")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1")
	board1 = newParts.BoardByName("board1_r1")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r1")
	board1 = newParts.BoardByName("board1_r2")
	test.That(t, board1.(*board.FakeBoard).Name, test.ShouldEqual, "board1_r2")
	test.That(t, newParts.BoardByName("board1_what"), test.ShouldBeNil)

	sensor1 := newParts.SensorByName("sensor1")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1")
	sensor1 = newParts.SensorByName("sensor1_r1")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r1")
	sensor1 = newParts.SensorByName("sensor1_r2")
	test.That(t, sensor1.(*fake.Compass).Name, test.ShouldEqual, "sensor1_r2")
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

func testPartsMergeAddModify(t *testing.T, add bool) {
	var modFunc func(rcv, toChange *robotParts) (*PartsMergeResult, error)
	if add {
		modFunc = (*robotParts).MergeAdd
	} else {
		modFunc = (*robotParts).MergeModify
	}

	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
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
		test.That(t, utils.NewStringSet(toCheck.LidarDeviceNames()...), test.ShouldBeEmpty)
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
		test.That(t, utils.NewStringSet(toCheck.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
	}
	result, err := modFunc(parts, newRobotParts(logger))
	test.That(t, err, test.ShouldBeNil)
	checkSame(parts)

	emptyParts := newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.AddRemote(setupInjectRobotWithSuffx(logger, "_other1"), api.RemoteConfig{Name: "other1"})
	result, err = modFunc(parts, otherParts)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2", "other1"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2", "arm1_other", "arm2_other", "arm1_other1", "arm2_other1"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2", "gripper1_other", "gripper2_other", "gripper1_other1", "gripper2_other1"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2", "camera1_other", "camera2_other", "camera1_other1", "camera2_other1"))
	test.That(t, utils.NewStringSet(parts.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2", "base1_other", "base2_other", "base1_other1", "base2_other1"))
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2", "board1_other", "board2_other", "board1_other1", "board2_other1"))
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2", "sensor1_other", "sensor2_other", "sensor1_other1", "sensor2_other1"))
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	emptyParts = newRobotParts(logger)
	test.That(t, result.Process(emptyParts), test.ShouldBeNil)
	checkEmpty(emptyParts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	sameParts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "1"}, false)
	test.That(t, err, test.ShouldBeNil)
	_, err = sameParts.processManager.AddProcess(context.Background(), &fakeProcess{id: "2"}, false)
	test.That(t, err, test.ShouldBeNil)

	result, err = modFunc(parts, sameParts)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, utils.NewStringSet(parts.RemoteNames()...), test.ShouldResemble, utils.NewStringSet("remote1", "remote2", "other1"))
	test.That(t, utils.NewStringSet(parts.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1", "arm2", "arm1_r1", "arm2_r1", "arm1_r2", "arm2_r2", "arm1_other", "arm2_other", "arm1_other1", "arm2_other1"))
	test.That(t, utils.NewStringSet(parts.GripperNames()...), test.ShouldResemble, utils.NewStringSet("gripper1", "gripper2", "gripper1_r1", "gripper2_r1", "gripper1_r2", "gripper2_r2", "gripper1_other", "gripper2_other", "gripper1_other1", "gripper2_other1"))
	test.That(t, utils.NewStringSet(parts.CameraNames()...), test.ShouldResemble, utils.NewStringSet("camera1", "camera2", "camera1_r1", "camera2_r1", "camera1_r2", "camera2_r2", "camera1_other", "camera2_other", "camera1_other1", "camera2_other1"))
	test.That(t, utils.NewStringSet(parts.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2", "lidar1_other", "lidar2_other", "lidar1_other1", "lidar2_other1"))
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
	test.That(t, utils.NewStringSet(emptyParts.LidarDeviceNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(emptyParts.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

	err = result.Process(parts)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected")
}

func TestPartsMergeAdd(t *testing.T) {
	testPartsMergeAddModify(t, true)
}

func TestPartsMergeModify(t *testing.T) {
	testPartsMergeAddModify(t, false)
}

func TestPartsMergeRemove(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
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
		test.That(t, utils.NewStringSet(toCheck.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2", "base1_r1", "base2_r1", "base1_r2", "base2_r2"))
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2", "board1_r1", "board2_r1", "board1_r2", "board2_r2"))
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor1", "sensor2", "sensor1_r1", "sensor2_r1", "sensor1_r2", "sensor2_r2"))
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
	}

	parts.MergeRemove(newRobotParts(logger))
	checkSame(parts)

	otherRobot := setupInjectRobotWithSuffx(logger, "_other")
	otherParts := partsForRemoteRobot(otherRobot)
	otherParts.AddRemote(setupInjectRobotWithSuffx(logger, "_other1"), api.RemoteConfig{Name: "other1"})
	parts.MergeRemove(otherParts)
	checkSame(parts)

	sameParts := partsForRemoteRobot(injectRobot)
	sameParts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	sameParts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
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
	test.That(t, utils.NewStringSet(parts.LidarDeviceNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.BaseNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.BoardNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.SensorNames()...), test.ShouldBeEmpty)
	test.That(t, utils.NewStringSet(parts.processManager.ProcessIDs()...), test.ShouldBeEmpty)
}

func TestPartsFilterFromConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	injectRobot := setupInjectRobot(logger)

	parts := partsForRemoteRobot(injectRobot)
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r1"), api.RemoteConfig{Name: "remote1"})
	parts.AddRemote(setupInjectRobotWithSuffx(logger, "_r2"), api.RemoteConfig{Name: "remote2"})
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
		test.That(t, utils.NewStringSet(toCheck.LidarDeviceNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(toCheck.processManager.ProcessIDs()...), test.ShouldBeEmpty)
	}

	filtered, err := parts.FilterFromConfig(&api.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	checkEmpty(filtered)

	filtered, err = parts.FilterFromConfig(&api.Config{
		Remotes: []api.RemoteConfig{
			{
				Name: "what",
			},
		},
		Components: []api.ComponentConfig{
			{
				Name: "what1",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "what2",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "what3",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "what4",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "what5",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "what6",
				Type: api.ComponentTypeSensor,
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

	_, err = parts.FilterFromConfig(&api.Config{
		Components: []api.ComponentConfig{
			{
				Name: "what1",
				Type: "something",
			},
		},
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")
	test.That(t, err.Error(), test.ShouldContainSubstring, "something")

	filtered, err = parts.FilterFromConfig(&api.Config{
		Components: []api.ComponentConfig{
			{
				Name: "arm2",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "gripper2",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "camera2",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "lidar2",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "base2",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "sensor2",
				Type: api.ComponentTypeSensor,
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
	test.That(t, utils.NewStringSet(filtered.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2"))
	test.That(t, utils.NewStringSet(filtered.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("2"))

	filtered, err = parts.FilterFromConfig(&api.Config{
		Remotes: []api.RemoteConfig{
			{
				Name: "remote2",
			},
		},
		Components: []api.ComponentConfig{
			{
				Name: "arm2",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "gripper2",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "camera2",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "lidar2",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "base2",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "sensor2",
				Type: api.ComponentTypeSensor,
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
	test.That(t, utils.NewStringSet(filtered.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar2", "lidar1_r2", "lidar2_r2"))
	test.That(t, utils.NewStringSet(filtered.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2", "base1_r2", "base2_r2"))
	test.That(t, utils.NewStringSet(filtered.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board2", "board1_r2", "board2_r2"))
	test.That(t, utils.NewStringSet(filtered.SensorNames()...), test.ShouldResemble, utils.NewStringSet("sensor2", "sensor1_r2", "sensor2_r2"))
	test.That(t, utils.NewStringSet(filtered.processManager.ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("2"))

	filtered, err = parts.FilterFromConfig(&api.Config{
		Remotes: []api.RemoteConfig{
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
		Components: []api.ComponentConfig{
			{
				Name: "arm1",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "arm2",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "arm3",
				Type: api.ComponentTypeArm,
			},
			{
				Name: "gripper1",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "gripper2",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "gripper3",
				Type: api.ComponentTypeGripper,
			},
			{
				Name: "camera1",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "camera2",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "camera3",
				Type: api.ComponentTypeCamera,
			},
			{
				Name: "lidar1",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "lidar2",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "lidar3",
				Type: api.ComponentTypeLidar,
			},
			{
				Name: "base1",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "base2",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "base3",
				Type: api.ComponentTypeBase,
			},
			{
				Name: "sensor1",
				Type: api.ComponentTypeSensor,
			},
			{
				Name: "sensor2",
				Type: api.ComponentTypeSensor,
			},
			{
				Name: "sensor3",
				Type: api.ComponentTypeSensor,
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
	test.That(t, utils.NewStringSet(filtered.LidarDeviceNames()...), test.ShouldResemble, utils.NewStringSet("lidar1", "lidar2", "lidar1_r1", "lidar2_r1", "lidar1_r2", "lidar2_r2"))
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
