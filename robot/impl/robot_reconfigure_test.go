package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/robots/fake"
	"go.viam.com/core/utils"

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
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(conf1, conf1)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
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

		robot.Reconfigure(newRobot, diff)
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
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("empty to additive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		robot, err := New(context.Background(), emptyConf, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(emptyConf, conf1)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
		}()

		robot.Reconfigure(newRobot, diff)
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
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf4 := ConfigFromFile(t, "data/diff_config_4.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), conf4, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(conf1, conf4)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
		}()

		robot.Reconfigure(newRobot, diff)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 1)

		analog1 := board1.AnalogReader("analog1")
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		analog2 := board1.AnalogReader("analog2")
		test.That(t, analog2.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("subtractive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), emptyConf, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(conf1, emptyConf)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
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

		robot.Reconfigure(newRobot, diff)
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
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), conf2, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(conf1, conf2)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
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

		analog1 := board1.AnalogReader("analog1")
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		robot.Reconfigure(newRobot, diff)
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
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 1)
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, analog1.(*board.FakeAnalog).ReconfigureCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1, test.ShouldEqual, arm1)

		newBase1 := robot.BaseByName("base1")
		test.That(t, newBase1, test.ShouldEqual, base1)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1, test.ShouldEqual, board1)

		newAnalog1 := newBoard1.AnalogReader("analog1")
		test.That(t, newAnalog1, test.ShouldEqual, analog1)

		newAnalog2 := newBoard1.AnalogReader("analog2")
		test.That(t, newAnalog2, test.ShouldNotEqual, analog1)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("mixed diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		newRobot, err := New(context.Background(), conf3, logger)
		test.That(t, err, test.ShouldBeNil)
		diff, err := config.DiffConfigs(conf1, conf3)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
			test.That(t, newRobot.Close(), test.ShouldBeNil)
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

		robot.Reconfigure(newRobot, diff)
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
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 0)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1, test.ShouldEqual, arm1)

		test.That(t, robot.BaseByName("base1"), test.ShouldBeNil)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1, test.ShouldEqual, board1)

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
}

func TestRobotReconfigureFromConfig(t *testing.T) {
	ConfigFromFile := func(t *testing.T, filePath string) *config.Config {
		conf, err := config.Read(filePath)
		test.That(t, err, test.ShouldBeNil)
		return conf
	}

	t.Run("no diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		robot, err := New(context.Background(), conf1, logger)
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

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf1), test.ShouldBeNil)
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

	t.Run("empty to additive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		robot, err := New(context.Background(), emptyConf, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, robot.ReconfigureFromConfig(context.Background(), emptyConf), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf1), test.ShouldBeNil)
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
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf4 := ConfigFromFile(t, "data/diff_config_4.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(), test.ShouldBeNil)
		}()

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf1), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf4), test.ShouldBeNil)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ArmNames()...), test.ShouldResemble, utils.NewStringSet("arm1"))
		test.That(t, utils.NewStringSet(robot.GripperNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.CameraNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.LidarNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.BaseNames()...), test.ShouldResemble, utils.NewStringSet("base1", "base2"))
		test.That(t, utils.NewStringSet(robot.BoardNames()...), test.ShouldResemble, utils.NewStringSet("board1"))
		test.That(t, utils.NewStringSet(robot.SensorNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1 := robot.ArmByName("arm1")
		test.That(t, arm1.(*fake.Arm).CloseCount, test.ShouldEqual, 0)

		base1 := robot.BaseByName("base1")
		test.That(t, base1.(*fake.Base).CloseCount, test.ShouldEqual, 0)

		board1 := robot.BoardByName("board1")
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)

		analog1 := board1.AnalogReader("analog1")
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		analog2 := board1.AnalogReader("analog2")
		test.That(t, analog2.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("subtractive diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		emptyConf := ConfigFromFile(t, "data/diff_config_empty.json")
		robot, err := New(context.Background(), conf1, logger)
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

		test.That(t, robot.ReconfigureFromConfig(context.Background(), emptyConf), test.ShouldBeNil)
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
		robot, err := New(context.Background(), conf1, logger)
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

		analog1 := board1.AnalogReader("analog1")
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 0)

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf2), test.ShouldBeNil)
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
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, analog1.(*board.FakeAnalog).CloseCount, test.ShouldEqual, 1)
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, analog1.(*board.FakeAnalog).ReconfigureCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1, test.ShouldEqual, arm1)

		newBase1 := robot.BaseByName("base1")
		test.That(t, newBase1, test.ShouldEqual, base1)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1, test.ShouldEqual, board1)

		newAnalog1 := newBoard1.AnalogReader("analog1")
		test.That(t, newAnalog1, test.ShouldEqual, analog1)

		newAnalog2 := newBoard1.AnalogReader("analog2")
		test.That(t, newAnalog2, test.ShouldNotEqual, analog1)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("mixed diff", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_3.json")
		robot, err := New(context.Background(), conf1, logger)
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

		test.That(t, robot.ReconfigureFromConfig(context.Background(), conf3), test.ShouldBeNil)
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
		test.That(t, board1.(*board.FakeBoard).CloseCount, test.ShouldEqual, 0)
		test.That(t, arm1.(*fake.Arm).ReconfigureCount, test.ShouldEqual, 1)
		test.That(t, base1.(*fake.Base).ReconfigureCount, test.ShouldEqual, 0)
		test.That(t, board1.(*board.FakeBoard).ReconfigureCount, test.ShouldEqual, 1)

		newArm1 := robot.ArmByName("arm1")
		test.That(t, newArm1, test.ShouldEqual, arm1)

		test.That(t, robot.BaseByName("base1"), test.ShouldBeNil)

		newBoard1 := robot.BoardByName("board1")
		test.That(t, newBoard1, test.ShouldEqual, board1)

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
		robot, err := New(context.Background(), conf1, logger)
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

		err = robot.ReconfigureFromConfig(context.Background(), conf3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error processing draft changes")
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

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
