package builtinrobot

import (
	"context"
	"testing"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/config"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestRobotReconfigure(t *testing.T) {
	ConfigFromFile := func(t *testing.T, filePath string) *config.Config {
		conf, err := config.Read(filePath)
		test.That(t, err, test.ShouldBeNil)
		return conf
	}

	t.Run("no diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		robot, err := NewRobot(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, robot.Reconfigure(context.Background(), conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("additive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		robot, err := NewRobot(context.Background(), emptyConf, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, robot.Reconfigure(context.Background(), emptyConf), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		test.That(t, robot.Reconfigure(context.Background(), conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("subtractive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		robot, err := NewRobot(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.Reconfigure(context.Background(), emptyConf), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		test.That(t, robot.ArmByName("arm1"), test.ShouldBeNil)
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 1)

		test.That(t, robot.BaseByName("base1"), test.ShouldBeNil)
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 1)

		test.That(t, robot.BoardByName("board1"), test.ShouldBeNil)
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 1)
	})

	t.Run("modificative diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf2 := ConfigFromFile(t, "data/diff_config_2.json")
		robot, err := NewRobot(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.Reconfigure(context.Background(), conf2), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 1)
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		newBase1 := robot.BaseByName("base1")
		test.That(t, newBase1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("mixed diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3.json")
		robot, err := NewRobot(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.Reconfigure(context.Background(), conf3), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base2"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1", "board2"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "3"))

		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 1)
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.BaseByName("base1"), test.ShouldBeNil)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		base2 := robot.BaseByName("base2")
		test.That(t, base2.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board2 := robot.BoardByName("board2")
		test.That(t, board2.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("3")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("rollback", func(t *testing.T) {
		// processing modify will fail
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3_bad.json")
		robot, err := NewRobot(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		err = robot.Reconfigure(context.Background(), conf3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "processing draft changes")
		test.That(t, err.Error(), test.ShouldContainSubstring, "unknown component")

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.ArmByName("arm1"), test.ShouldEqual, arm1)
		test.That(t, robot.BaseByName("base1"), test.ShouldEqual, base1)
		test.That(t, robot.BoardByName("board1"), test.ShouldEqual, board1)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})
}
