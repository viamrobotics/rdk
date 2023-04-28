package robotimpl

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a8m/envsubst"
	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/sensors"
	_ "go.viam.com/rdk/services/sensors/builtin"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

var (
	// these settings to be toggled in test cases specifically
	// testing for a reconfigurability mismatch.
	reconfigurableTrue        = true
	testReconfiguringMismatch = false
)

func TestRobotReconfigure(t *testing.T) {
	test.That(t, len(resource.DefaultServices()), test.ShouldEqual, 3)
	ConfigFromFile := func(t *testing.T, filePath string) *config.Config {
		t.Helper()
		logger := golog.NewTestLogger(t)
		buf, err := envsubst.ReadFile(filePath)
		test.That(t, err, test.ShouldBeNil)
		conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
		test.That(t, err, test.ShouldBeNil)
		return conf
	}
	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	modelName2 := utils.RandomAlphaString(5)
	test.That(t, os.Setenv("TEST_MODEL_NAME_1", modelName1), test.ShouldBeNil)
	test.That(t, os.Setenv("TEST_MODEL_NAME_2", modelName2), test.ShouldBeNil)

	resource.RegisterComponent(mockAPI, resource.DefaultModelFamily.WithModel(modelName1),
		resource.Registration[resource.Resource, *mockFakeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				// test if implicit depencies are properly propagated
				for _, dep := range conf.ConvertedAttributes.(*mockFakeConfig).InferredDep {
					if _, ok := deps[mockNamed(dep)]; !ok {
						return nil, errors.Errorf("inferred dependency %q cannot be found", mockNamed(dep))
					}
				}
				if conf.ConvertedAttributes.(*mockFakeConfig).ShouldFail {
					return nil, errors.Errorf("cannot build %q for some obscure reason", conf.Name)
				}
				return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
			},
		})

	resetComponentFailureState := func() {
		reconfigurableTrue = true
		testReconfiguringMismatch = false
	}
	resource.RegisterComponent(mockAPI, resource.DefaultModelFamily.WithModel(modelName2),
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (resource.Resource, error) {
				if reconfigurableTrue && testReconfiguringMismatch {
					reconfigurableTrue = false
					return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
				}
				return &mockFake2{Named: conf.ResourceName().AsNamed()}, nil
			},
		})

	t.Run("no diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")

		ctx := context.Background()
		robot, err := New(ctx, conf1, logger)
		test.That(t, err, test.ShouldBeNil)

		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		resources := robot.ResourceNames()
		test.That(t, len(resources), test.ShouldEqual, 8)

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		names := rdktestutils.NewResourceNameSet(robot.ResourceNames()...)
		names2 := rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		)
		_ = names2
		_ = names
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				mockNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		robot.Reconfigure(ctx, conf1)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				mockNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(arm.Named("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(board.Named("board1"))
		test.That(t, err, test.ShouldBeNil)

		mock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake2).reconfCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("reconfiguring unreconfigurable", func(t *testing.T) {
		resetComponentFailureState()
		testReconfiguringMismatch = true
		// processing modify will fail
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf3 := ConfigFromFile(t, "data/diff_config_4_bad.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				mockNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm1, err := arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		base1, err := base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		board1, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		resource1, err := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, err, test.ShouldBeNil)

		mock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		reconfigurableTrue = false
		robot.Reconfigure(context.Background(), conf3)

		_, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)

		reconfigurableTrue = true

		rr, ok := robot.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)

		rr.triggerConfig <- struct{}{}

		utils.SelectContextOrWait(context.Background(), 200*time.Millisecond)

		_, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)

		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				mockNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		newArm1, err := arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newArm1, test.ShouldEqual, arm1)

		newBase1, err := base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newBase1, test.ShouldEqual, base1)

		newBoard1, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newBoard1, test.ShouldEqual, board1)

		newResource1, err := robot.ResourceByName(arm.Named("arm1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newResource1, test.ShouldEqual, resource1)

		newMock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newMock1, test.ShouldEqual, mock1)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		testReconfiguringMismatch = false
	})

	t.Run("additive deps diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_deps1.json")
		conf2 := ConfigFromFile(t, "data/diff_config_deps10.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"),
		}

		robot.Reconfigure(context.Background(), conf1)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(motor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				resource.DefaultServices(),
				mockNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		robot.Reconfigure(context.Background(), conf2)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				mockNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldBeNil)

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 1000)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		rdktestutils.VerifyTopologicallySortedLevels(
			t,
			robot.(*localRobot).manager.resources,
			[][]resource.Name{
				rdktestutils.ConcatResourceNames(
					motorNames,
					resource.DefaultServices(),
					[]resource.Name{mockNamed("mock1")}),
				rdktestutils.ConcatResourceNames(
					armNames,
					[]resource.Name{mockNamed("mock2"), mockNamed("mock3")}),
				baseNames,
				boardNames,
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("modificative deps diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf3 := ConfigFromFile(t, "data/diff_config_deps3.json")
		conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		robot, err := New(context.Background(), conf3, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf3)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("5")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 4000)
		_, ok := b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeFalse)

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldBeNil)

		b, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		_, err = b.GPIOPinByName("5")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err = pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 4000)
		pin, err = b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err = pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 1000) // TODO double check this is the expected result
		_, ok = b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		rdktestutils.VerifyTopologicallySortedLevels(
			t,
			robot.(*localRobot).manager.resources,
			[][]resource.Name{
				rdktestutils.ConcatResourceNames(
					motorNames,
					resource.DefaultServices()),
				armNames,
				baseNames,
				boardNames,
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("deletion deps diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		conf4 := ConfigFromFile(t, "data/diff_config_deps4.json")
		robot, err := New(context.Background(), conf2, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		arm2, err := arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldBeNil)

		test.That(t, arm2.(*fake.Arm).CloseCount, test.ShouldEqual, 0)
		robot.Reconfigure(context.Background(), conf4)
		test.That(t, arm2.(*fake.Arm).CloseCount, test.ShouldEqual, 1)

		boardNames = []resource.Name{board.Named("board1"), board.Named("board2")}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				[]resource.Name{
					mockNamed("mock6"),
				},
			)...))
	})

	t.Run("mixed deps diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		conf6 := ConfigFromFile(t, "data/diff_config_deps6.json")
		robot, err := New(context.Background(), conf2, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))
		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 1000)
		_, ok := b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)

		armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm3")}
		baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames = []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m4"), motor.Named("m5")}
		boardNames = []resource.Name{
			board.Named("board1"),
			board.Named("board2"), board.Named("board3"),
		}

		motor2, err := motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldBeNil)

		robot.Reconfigure(context.Background(), conf6)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		_, err = arm.FromRobot(robot, "arm3")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldBeNil)

		nextMotor2, err := motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldBeNil)
		// m2 lost its dependency on arm2 after looking conf6
		// but only relies on base1 so it should never have been
		// removed but only reconfigured.
		test.That(t, nextMotor2, test.ShouldPointTo, motor2)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m5")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldBeNil)

		b, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err = b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err = pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 0)
		_, ok = b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = b.DigitalInterruptByName("encoderC")
		test.That(t, ok, test.ShouldBeTrue)

		_, err = board.FromRobot(robot, "board3")
		test.That(t, err, test.ShouldBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		rdktestutils.VerifyTopologicallySortedLevels(
			t,
			robot.(*localRobot).manager.resources,
			[][]resource.Name{
				rdktestutils.ConcatResourceNames(
					motorNames,
					resource.DefaultServices(),
					[]resource.Name{arm.Named("arm1")},
				),
				{
					arm.Named("arm3"),
					base.Named("base1"),
					board.Named("board3"),
				},
				{
					base.Named("base2"),
					board.Named("board2"),
				},
				{board.Named("board1")},
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("from empty conf with deps", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		cempty := ConfigFromFile(t, "data/diff_config_empty.json")
		conf6 := ConfigFromFile(t, "data/diff_config_deps6.json")
		ctx := context.Background()
		robot, err := New(ctx, cempty, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		resources := robot.ResourceNames()
		test.That(t, len(resources), test.ShouldEqual, 3)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(arm.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(base.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(board.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(
			t,
			rdktestutils.NewResourceNameSet(robot.ResourceNames()...),
			test.ShouldResemble,
			rdktestutils.NewResourceNameSet(resource.DefaultServices()...),
		)
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldBeEmpty)

		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm3")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m4"), motor.Named("m5")}
		boardNames := []resource.Name{
			board.Named("board1"),
			board.Named("board2"), board.Named("board3"),
		}
		robot.Reconfigure(context.Background(), conf6)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(baseNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				motorNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldBeNil)

		_, err = arm.FromRobot(robot, "arm3")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m5")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldBeNil)

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 0)
		_, ok := b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = b.DigitalInterruptByName("encoderC")
		test.That(t, ok, test.ShouldBeTrue)

		_, err = board.FromRobot(robot, "board3")
		test.That(t, err, test.ShouldBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		rdktestutils.VerifyTopologicallySortedLevels(
			t,
			robot.(*localRobot).manager.resources,
			[][]resource.Name{
				rdktestutils.ConcatResourceNames(
					motorNames,
					resource.DefaultServices(),
					[]resource.Name{arm.Named("arm1")},
				),
				{
					arm.Named("arm3"),
					base.Named("base1"),
					board.Named("board3"),
				},
				{
					base.Named("base2"),
					board.Named("board2"),
				},
				{board.Named("board1")},
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("incremental deps config", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf4 := ConfigFromFile(t, "data/diff_config_deps4.json")
		conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		robot, err := New(context.Background(), conf4, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		motorNames := []resource.Name{motor.Named("m1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
			mockNamed("mock6"),
		}
		encoderNames := []resource.Name{encoder.Named("e1")}

		robot.Reconfigure(context.Background(), conf7)
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
				encoderNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		mock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock6, err := robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				resource.DefaultServices(),
				boardNames,
				mockNames,
				encoderNames,
			)...))
	})

	t.Run("parent attribute change deps config", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		conf8 := ConfigFromFile(t, "data/diff_config_deps8.json")
		robot, err := New(context.Background(), conf7, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		motorNames := []resource.Name{motor.Named("m1")}
		encoderNames := []resource.Name{encoder.Named("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"), mockNamed("mock6"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				encoderNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		eA, ok := b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)
		eB, ok := b.DigitalInterruptByName("encoder-b")
		test.That(t, ok, test.ShouldBeTrue)

		m, err := motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

		test.That(t, eA.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eB.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eA.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			c, err = m.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, c, test.ShouldEqual, 1)
		})

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		mock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				resource.DefaultServices(),
				boardNames,
				mockNames,
				encoderNames,
			)...))
		robot.Reconfigure(context.Background(), conf8)
		mockNames = []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(base.NamesFromRobot(robot)...),
			test.ShouldBeEmpty,
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)
		test.That(t, utils.NewStringSet(camera.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(gripper.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(sensor.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, utils.NewStringSet(servo.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
				encoderNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		b, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		eA, ok = b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)
		eB, ok = b.DigitalInterruptByName("encoder-b")
		test.That(t, ok, test.ShouldBeTrue)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		t.Log("the underlying pins changed but not the encoder names, so we keep the value")
		test.That(t, c, test.ShouldEqual, 1)

		test.That(t, eB.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eA.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			c, err = m.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, c, test.ShouldEqual, 2)
		})

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		mock1, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock4, err = robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err = robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("child component fails dep", func(t *testing.T) {
		resetComponentFailureState()
		testReconfiguringMismatch = true
		reconfigurableTrue = true
		logger := golog.NewTestLogger(t)
		conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		conf9 := ConfigFromFile(t, "data/diff_config_deps9_bad.json")
		robot, err := New(context.Background(), conf7, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		motorNames := []resource.Name{motor.Named("m1")}
		encoderNames := []resource.Name{encoder.Named("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
			mockNamed("mock6"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)

		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
				encoderNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		eA, ok := b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)
		eB, ok := b.DigitalInterruptByName("encoder-b")
		test.That(t, ok, test.ShouldBeTrue)

		m, err := motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

		test.That(t, eA.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eB.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eA.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			c, err = m.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, c, test.ShouldEqual, 1)
		})

		_, err = motor.FromRobot(robot, "m2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m3")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = motor.FromRobot(robot, "m4")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = base.FromRobot(robot, "base2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		mock1, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock6, err := robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				resource.DefaultServices(),
				boardNames,
				mockNames,
				encoderNames,
			)...))

		reconfigurableTrue = false
		robot.Reconfigure(context.Background(), conf9)

		mockNames = []resource.Name{
			mockNamed("mock2"),
			mockNamed("mock3"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)

		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
				encoderNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		b, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		eA, ok = b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)
		eB, ok = b.DigitalInterruptByName("encoder-b")
		test.That(t, ok, test.ShouldBeTrue)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 1)

		test.That(t, eA.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eB.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eA.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			c, err = m.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, c, test.ShouldEqual, 2)
		})

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldNotBeNil)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, err = robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldNotBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted = robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				resource.DefaultServices(),
				boardNames,
				mockNames,
				encoderNames,
				[]resource.Name{
					arm.Named("armFake"),
					mockNamed("mock1"),
					mockNamed("mock4"),
					mockNamed("mock5"),
					mockNamed("mock6"),
				},
			)...))
		conf9good := ConfigFromFile(t, "data/diff_config_deps9_good.json")
		robot.Reconfigure(context.Background(), conf9good)

		mockNames = []resource.Name{
			mockNamed("mock2"), mockNamed("mock1"), mockNamed("mock3"),
			mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)
		test.That(
			t,
			utils.NewStringSet(motor.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(motorNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(board.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(boardNames...)...),
		)
		test.That(
			t,
			utils.NewStringSet(encoder.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(encoderNames...)...),
		)

		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				resource.DefaultServices(),
				motorNames,
				mockNames,
				encoderNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		b, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		eA, ok = b.DigitalInterruptByName("encoder")
		test.That(t, ok, test.ShouldBeTrue)
		eB, ok = b.DigitalInterruptByName("encoder-b")
		test.That(t, ok, test.ShouldBeTrue)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 2)

		test.That(t, eA.Tick(context.Background(), false, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eB.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)
		test.That(t, eA.Tick(context.Background(), true, uint64(time.Now().UnixNano())), test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			c, err = m.Position(context.Background(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, c, test.ShouldEqual, 3)
		})

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		mock1, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock4, err = robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock5, err = robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldNotBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		reconfigurableTrue = true

		rr, ok := robot.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)

		rr.triggerConfig <- struct{}{}

		utils.SelectContextOrWait(context.Background(), 200*time.Millisecond)
		mock6, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldBeNil)

		sorted = robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				resource.DefaultServices(),
				boardNames,
				mockNames,
				encoderNames,
				[]resource.Name{
					arm.Named("armFake"),
					mockNamed("mock6"),
				},
			)...))
	})
	t.Run("complex diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		conf1 := ConfigFromFile(t, "data/diff_config_deps11.json")
		conf2 := ConfigFromFile(t, "data/diff_config_deps12.json")
		robot, err := New(context.Background(), conf1, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		armNames := []resource.Name{arm.Named("mock7")}
		mockNames := []resource.Name{
			mockNamed("mock3"), mockNamed("mock4"),
			mockNamed("mock6"), mockNamed("mock5"),
		}

		robot.Reconfigure(context.Background(), conf1)
		test.That(
			t,
			utils.NewStringSet(arm.NamesFromRobot(robot)...),
			test.ShouldResemble,
			utils.NewStringSet(rdktestutils.ExtractNames(armNames...)...),
		)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				resource.DefaultServices(),
				mockNames,
			)...))
		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldNotBeNil)
		_, err = arm.FromRobot(robot, "mock7")
		test.That(t, err, test.ShouldBeNil)

		robot.Reconfigure(context.Background(), conf2)
		mockNames = []resource.Name{
			mockNamed("mock1"),
			mockNamed("mock3"), mockNamed("mock2"), mockNamed("mock5"),
		}
		test.That(t, utils.NewStringSet(robot.RemoteNames()...), test.ShouldBeEmpty)

		test.That(t, utils.NewStringSet(arm.NamesFromRobot(robot)...), test.ShouldBeEmpty)
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				mockNames,
				resource.DefaultServices(),
			)...))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)
		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("test processes", func(t *testing.T) {
		resetComponentFailureState()
		logger := golog.NewTestLogger(t)
		tempDir := t.TempDir()
		robot, err := New(context.Background(), &config.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()
		// create a unexecutable file
		noExecF, err := os.CreateTemp(tempDir, "noexec*.sh")
		test.That(t, err, test.ShouldBeNil)
		err = noExecF.Close()
		test.That(t, err, test.ShouldBeNil)
		// create a origin file
		originF, err := os.CreateTemp(tempDir, "origin*")
		test.That(t, err, test.ShouldBeNil)
		token := make([]byte, 128)
		_, err = rand.Read(token)
		test.That(t, err, test.ShouldBeNil)
		_, err = originF.Write(token)
		test.That(t, err, test.ShouldBeNil)
		err = originF.Sync()
		test.That(t, err, test.ShouldBeNil)
		// create a target file
		targetF, err := os.CreateTemp(tempDir, "target*")
		test.That(t, err, test.ShouldBeNil)

		// create a second target file
		target2F, err := os.CreateTemp(tempDir, "target*")
		test.That(t, err, test.ShouldBeNil)

		// config1
		config1 := &config.Config{
			Processes: []pexec.ProcessConfig{
				{
					ID:      "shouldfail", // this process won't be executed
					Name:    "false",
					OneShot: true,
				},
				{
					ID:      "noexec", // file exist but exec bit not set
					Name:    noExecF.Name(),
					OneShot: true,
				},
				{
					ID:   "shouldsuceed", // this keep succeeding
					Name: "true",
				},
				{
					ID:      "noexist", // file doesn't exists
					Name:    fmt.Sprintf("%s/%s", tempDir, "noexistfile"),
					OneShot: true,
					Log:     true,
				},
				{
					ID:   "filehandle", // this keep succeeding and will be changed
					Name: "true",
				},
				{
					ID:   "touch", // touch a file
					Name: "sh",
					CWD:  tempDir,
					Args: []string{
						"-c",
						"sleep 0.4;touch afile",
					},
					OneShot: true,
				},
			},
		}
		robot.Reconfigure(context.Background(), config1)
		_, ok := robot.ProcessManager().ProcessByID("shouldfail")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("shouldsuceed")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("noexist")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("noexec")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("filehandle")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("touch")
		test.That(t, ok, test.ShouldBeTrue)
		utils.SelectContextOrWait(context.Background(), 1*time.Second)
		_, err = os.Stat(fmt.Sprintf("%s/%s", tempDir, "afile"))
		test.That(t, err, test.ShouldBeNil)
		config2 := &config.Config{
			Processes: []pexec.ProcessConfig{
				{
					ID:      "shouldfail", // now it succeeds
					Name:    "true",
					OneShot: true,
				},
				{
					ID:      "shouldsuceed", // now it fails
					Name:    "false",
					OneShot: true,
				},
				{
					ID:   "filehandle", // this transfer originF to targetF after 2s
					Name: "sh",
					Args: []string{
						"-c",
						fmt.Sprintf("sleep 2; cat %s >> %s", originF.Name(), targetF.Name()),
					},
					OneShot: true,
				},
				{
					ID:   "filehandle2", // this transfer originF to targetF after 2s
					Name: "sh",
					Args: []string{
						"-c",
						fmt.Sprintf("sleep 0.4;cat %s >> %s", originF.Name(), target2F.Name()),
					},
				},
				{
					ID:   "remove", // remove the file
					Name: "sh",
					CWD:  tempDir,
					Args: []string{
						"-c",
						"sleep 0.2;rm afile",
					},
					OneShot: true,
					Log:     true,
				},
			},
		}
		robot.Reconfigure(context.Background(), config2)
		_, ok = robot.ProcessManager().ProcessByID("shouldfail")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("shouldsuceed")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("noexist")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("noexec")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("filehandle")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("touch")
		test.That(t, ok, test.ShouldBeFalse)
		_, ok = robot.ProcessManager().ProcessByID("remove")
		test.That(t, ok, test.ShouldBeTrue)
		r := make([]byte, 128)
		n, err := targetF.Read(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, 128)
		utils.SelectContextOrWait(context.Background(), 3*time.Second)
		_, err = targetF.Seek(0, 0)
		test.That(t, err, test.ShouldBeNil)
		n, err = targetF.Read(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, 128)
		test.That(t, r, test.ShouldResemble, token)
		utils.SelectContextOrWait(context.Background(), 3*time.Second)
		_, err = targetF.Read(r)
		test.That(t, err, test.ShouldNotBeNil)
		err = originF.Close()
		test.That(t, err, test.ShouldBeNil)
		err = targetF.Close()
		test.That(t, err, test.ShouldBeNil)
		stat, err := target2F.Stat()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stat.Size(), test.ShouldBeGreaterThan, 128)
		err = target2F.Close()
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(fmt.Sprintf("%s/%s", tempDir, "afile"))
		test.That(t, err, test.ShouldNotBeNil)
	})
}

// this serves as a test for updateWeakDependents as the sensors service defines a weak
// dependency.
func TestSensorsServiceReconfigure(t *testing.T) {
	logger := golog.NewTestLogger(t)

	emptyCfg, err := config.Read(context.Background(), "data/diff_config_empty.json", logger)
	test.That(t, err, test.ShouldBeNil)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	sensorNames := []resource.Name{movementsensor.Named("movement_sensor1"), movementsensor.Named("movement_sensor2")}

	t.Run("empty to two sensors", func(t *testing.T) {
		robot, err := New(context.Background(), emptyCfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))
	})

	t.Run("two sensors to empty", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))

		robot.Reconfigure(context.Background(), emptyCfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)
	})

	t.Run("two sensors to two sensors", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))
	})
}

type someTypeWithWeakAndStrongDeps struct {
	resource.Named
	resource.TriviallyCloseable
	resources resource.Dependencies
}

func (s *someTypeWithWeakAndStrongDeps) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	s.resources = deps
	ourConf, err := resource.NativeConfig[*someTypeWithWeakAndStrongDepsConfig](conf)
	if err != nil {
		return err
	}
	for _, dep := range ourConf.deps {
		if _, err := deps.Lookup(dep); err != nil {
			return err
		}
	}
	for _, dep := range ourConf.weakDeps {
		if _, err := deps.Lookup(dep); err != nil {
			return err
		}
	}
	return nil
}

type someTypeWithWeakAndStrongDepsConfig struct {
	deps     []resource.Name
	weakDeps []resource.Name
}

func (s *someTypeWithWeakAndStrongDepsConfig) Validate(_ string) ([]string, error) {
	depNames := make([]string, 0, len(s.deps))
	for _, dep := range s.deps {
		depNames = append(depNames, dep.String())
	}
	return depNames, nil
}

func TestUpdateWeakDependents(t *testing.T) {
	logger := golog.NewTestLogger(t)

	var emptyCfg config.Config
	test.That(t, emptyCfg.Ensure(false, logger), test.ShouldBeNil)

	robot, err := New(context.Background(), &emptyCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	}()

	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weaktype")
	weakModel := resource.NewModel(uuid.NewString(), "soweak", "weak1000")
	weak1Name := resource.NewName(weakAPI, "weak1")
	base1Name := base.Named("base1")

	resource.Register(
		weakAPI,
		weakModel,
		resource.Registration[*someTypeWithWeakAndStrongDeps, *someTypeWithWeakAndStrongDepsConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (*someTypeWithWeakAndStrongDeps, error) {
				return &someTypeWithWeakAndStrongDeps{
					Named:     conf.ResourceName().AsNamed(),
					resources: deps,
				}, nil
			},
			WeakDependencies: []internal.ResourceMatcher{internal.ComponentDependencyWildcardMatcher},
		})

	weakCfg1 := config.Config{
		Components: []resource.Config{
			{
				Name:      weak1Name.Name,
				API:       weakAPI,
				Model:     weakModel,
				DependsOn: []string{base1Name.Name},
			},
		},
	}
	test.That(t, weakCfg1.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg1)

	_, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unresolved dependencies")

	weakCfg2 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg2.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg2)

	res, err := robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.resources, test.ShouldHaveLength, 1)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)

	arm1Name := arm.Named("arm1")
	weakCfg3 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
			{
				Name:                arm1Name.Name,
				API:                 arm.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, weakCfg3.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg3)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.resources, test.ShouldContainKey, arm1Name)

	weakCfg4 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg4.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg4)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.resources, test.ShouldHaveLength, 1)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)

	base2Name := base.Named("base2")
	weakCfg5 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{
					deps: []resource.Name{generic.Named("foo")},
				},
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg5.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg5)

	_, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")

	weakCfg6 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{
					weakDeps: []resource.Name{base1Name},
				},
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg6.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg6)
	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.resources, test.ShouldContainKey, base2Name)

	weakCfg7 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{
					deps:     []resource.Name{base2Name},
					weakDeps: []resource.Name{base1Name},
				},
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg7.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg7)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.resources, test.ShouldContainKey, base2Name)
}

func TestDefaultServiceReconfigure(t *testing.T) {
	logger := golog.NewTestLogger(t)

	dmName := "dm"
	cfg1 := &config.Config{
		Services: []resource.Config{
			{
				Name:  dmName,
				API:   datamanager.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}
	robot, err := New(context.Background(), cfg1, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
	}()

	test.That(
		t,
		rdktestutils.NewResourceNameSet(robot.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			datamanager.Named(dmName),
			sensors.Named(resource.DefaultServiceName),
		),
	)
	sName := "sensors"
	cfg2 := &config.Config{
		Services: []resource.Config{
			{
				Name:  sName,
				API:   sensors.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}
	robot.Reconfigure(context.Background(), cfg2)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(robot.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			sensors.Named(sName),
		),
	)
}

func TestStatusServiceUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	emptyCfg, err := config.Read(context.Background(), "data/diff_config_empty.json", logger)
	test.That(t, err, test.ShouldBeNil)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	resourceNames := []resource.Name{
		movementsensor.Named("movement_sensor1"),
		movementsensor.Named("movement_sensor2"),
	}
	expected := map[resource.Name]interface{}{
		movementsensor.Named("movement_sensor1"): map[string]interface{}{},
		movementsensor.Named("movement_sensor2"): map[string]interface{}{},
	}

	t.Run("empty to not empty", func(t *testing.T) {
		robot, err := New(context.Background(), emptyCfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		_, err = robot.Status(context.Background(), resourceNames)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		robot.Reconfigure(context.Background(), cfg)

		statuses, err := robot.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statuses), test.ShouldEqual, 2)
		test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
		test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])
	})

	t.Run("not empty to empty", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		statuses, err := robot.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statuses), test.ShouldEqual, 2)
		test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
		test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])

		robot.Reconfigure(context.Background(), emptyCfg)

		_, err = robot.Status(context.Background(), resourceNames)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	})

	t.Run("no change", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		statuses, err := robot.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statuses), test.ShouldEqual, 2)
		test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
		test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])

		robot.Reconfigure(context.Background(), cfg)

		statuses, err = robot.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statuses), test.ShouldEqual, 2)
		test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
		test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])
	})
}

func TestRemoteRobotsGold(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	remote1, err := New(ctx, cfg, logger.Named("remote1"))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, remote1.Close(context.Background()), test.ShouldBeNil)
	}()

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = remote1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remote2, err := New(ctx, cfg, logger.Named("remote2"))
	test.That(t, err, test.ShouldBeNil)

	options, listener2, addr2 := robottestutils.CreateBaseOptionsAndListener(t)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"foo:pieceGripper"},
			},
			{
				Name:  "arm2",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"bar:pieceArm"},
			},
		},
		Services: []resource.Config{},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr2,
			},
		},
	}
	r, err := New(ctx, localConfig, logger.Named("local"))
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			arm.Named("arm1"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			motion.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
		),
	)
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	rr, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}

	utils.SelectContextOrWait(ctx, 2*time.Second)

	expectedSet := rdktestutils.NewResourceNameSet(
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("arm2"),
		arm.Named("foo:pieceArm"),
		audioinput.Named("foo:mic1"),
		camera.Named("foo:cameraOver"),
		movementsensor.Named("foo:movement_sensor1"),
		movementsensor.Named("foo:movement_sensor2"),
		gripper.Named("foo:pieceGripper"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		audioinput.Named("bar:mic1"),
		camera.Named("bar:cameraOver"),
		movementsensor.Named("bar:movement_sensor1"),
		movementsensor.Named("bar:movement_sensor2"),
		gripper.Named("bar:pieceGripper"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		datamanager.Named("bar:builtin"),
	)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)

	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	utils.SelectContextOrWait(ctx, 15*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			arm.Named("arm1"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			motion.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
		),
	)

	remote3, err := New(ctx, cfg, logger.Named("remote3"))
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, remote3.Close(context.Background()), test.ShouldBeNil)
	}()

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener2, err = net.Listen("tcp", listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener2
	err = remote3.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 5*time.Second)

	rr, ok = r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}

	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)
}

func TestInferRemoteRobotDependencyConnectAtStartup(t *testing.T) {
	logger := golog.NewTestLogger(t)

	fooCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	foo, err := New(ctx, fooCfg, logger.Named("foo"))
	test.That(t, err, test.ShouldBeNil)

	options, listener1, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
		},
	}
	r, err := New(ctx, localConfig, logger.Named("local"))
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
			arm.Named("arm1"),
			arm.Named("foo:pieceArm"),
			motion.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
		),
	)

	rr, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	expectedSet := rdktestutils.NewResourceNameSet(
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
	)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)

	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)
	// wait for local_robot to detect that the remote is now offline
	utils.SelectContextOrWait(ctx, 15*time.Second)
	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
		),
	)

	foo2, err := New(ctx, fooCfg, logger.Named("foo2"))
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, foo2.Close(context.Background()), test.ShouldBeNil)
	}()

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener1, err = net.Listen("tcp", listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener1
	err = foo2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 5*time.Second)

	rr, ok = r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}

	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)
}

func TestInferRemoteRobotDependencyConnectAfterStartup(t *testing.T) {
	logger := golog.NewTestLogger(t)

	fooCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	foo, err := New(ctx, fooCfg, logger.Named("foo"))
	test.That(t, err, test.ShouldBeNil)

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
		},
	}
	r, err := New(ctx, localConfig, logger.Named("local"))
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
		),
	)
	err = foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	rr, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	utils.SelectContextOrWait(ctx, 5*time.Second)
	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	expectedSet := rdktestutils.NewResourceNameSet(
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
	)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)

	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)
	// wait for local_robot to detect that the remote is now offline
	utils.SelectContextOrWait(ctx, 5*time.Second)
	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			datamanager.Named(resource.DefaultServiceName),
		),
	)
}

func TestInferRemoteRobotDependencyAmbiguous(t *testing.T) {
	logger := golog.NewTestLogger(t)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	foo, err := New(ctx, remoteCfg, logger.Named("foo"))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, foo.Close(context.Background()), test.ShouldBeNil)
	}()

	bar, err := New(ctx, remoteCfg, logger.Named("bar"))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, bar.Close(context.Background()), test.ShouldBeNil)
	}()

	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = foo.StartWeb(ctx, options1)
	test.That(t, err, test.ShouldBeNil)

	options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	err = bar.StartWeb(ctx, options2)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr2,
			},
		},
	}
	r, err := New(ctx, localConfig, logger.Named("local"))
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	expectedSet := rdktestutils.NewResourceNameSet(
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		datamanager.Named("bar:builtin"),
	)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)

	rr, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	// we expect the robot to correctly detect the ambiguous dependency and not build the resource
	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)

	// now reconfig with a fully qualified name
	reConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"foo:pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr2,
			},
		},
	}
	r.Reconfigure(ctx, reConfig)
	utils.SelectContextOrWait(ctx, 5*time.Second)
	rr.triggerConfig <- struct{}{}
	utils.SelectContextOrWait(ctx, 2*time.Second)

	finalSet := rdktestutils.NewResourceNameSet(
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		datamanager.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		datamanager.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		datamanager.Named("bar:builtin"),
		arm.Named("arm1"),
	)

	test.That(t, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, finalSet)
}

func TestReconfigureModelRebuild(t *testing.T) {
	logger := golog.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return &mockFake{Named: conf.ResourceName().AsNamed(), shouldRebuild: true}, nil
		},
	})

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model1,
				API:   mockAPI,
			},
		},
	}

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	name1 := mockNamed("one")
	res1, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 0)

	r.Reconfigure(ctx, cfg)
	res2, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldEqual, res1)
	test.That(t, res2.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res2.(*mockFake).closeCount, test.ShouldEqual, 0)

	newCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                "one",
				Model:               model1,
				API:                 mockAPI,
				ConvertedAttributes: resource.NoNativeConfig{},
			},
		},
	}

	r.Reconfigure(ctx, newCfg)
	res3, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res3, test.ShouldNotEqual, res1)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 1)
	test.That(t, res3.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res3.(*mockFake).closeCount, test.ShouldEqual, 0)
}

func TestReconfigureModelSwitch(t *testing.T) {
	logger := golog.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	modelName2 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)
	model2 := resource.DefaultModelFamily.WithModel(modelName2)

	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
		},
	})
	resource.RegisterComponent(mockAPI, model2, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return &mockFake2{Named: conf.ResourceName().AsNamed()}, nil
		},
	})

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model1,
				API:   mockAPI,
			},
		},
	}

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	name1 := mockNamed("one")
	res1, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 0)

	r.Reconfigure(ctx, cfg)
	res2, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldEqual, res1)
	test.That(t, res2.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res2.(*mockFake).closeCount, test.ShouldEqual, 0)

	newCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                "one",
				Model:               model2,
				API:                 mockAPI,
				ConvertedAttributes: resource.NoNativeConfig{},
			},
		},
	}

	r.Reconfigure(ctx, newCfg)
	res3, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res3, test.ShouldNotEqual, res1)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 1)
	test.That(t, res3.(*mockFake2).reconfCount, test.ShouldEqual, 0)
	test.That(t, res3.(*mockFake2).closeCount, test.ShouldEqual, 0)
}

func TestReconfigureRename(t *testing.T) {
	logger := golog.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	var logicalClock atomic.Int64

	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return &mockFake{
				Named:        conf.ResourceName().AsNamed(),
				logicalClock: &logicalClock,
				createdAt:    int(logicalClock.Add(1)),
			}, nil
		},
	})

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model1,
				API:   mockAPI,
			},
		},
	}

	ctx := context.Background()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	name1 := mockNamed("one")
	name2 := mockNamed("two")
	res1, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).createdAt, test.ShouldEqual, 1)

	newCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                "two",
				Model:               model1,
				API:                 mockAPI,
				ConvertedAttributes: resource.NoNativeConfig{},
			},
		},
	}

	r.Reconfigure(ctx, newCfg)
	res2, err := r.ResourceByName(name2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldNotEqual, res1)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 1)
	test.That(t, res1.(*mockFake).closedAt, test.ShouldEqual, 2)
	test.That(t, res2.(*mockFake).createdAt, test.ShouldEqual, 3)
	test.That(t, res2.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res2.(*mockFake).closeCount, test.ShouldEqual, 0)
}

type mockFake struct {
	resource.Named
	createdAt      int
	reconfCount    int
	reconfiguredAt int64
	failCount      int
	shouldRebuild  bool
	closeCount     int
	closedAt       int64
	logicalClock   *atomic.Int64
}

type mockFakeConfig struct {
	InferredDep           []string `json:"inferred_dep"`
	ShouldFail            bool     `json:"should_fail"`
	ShouldFailReconfigure int      `json:"should_fail_reconfigure"`
	Blah                  int      `json:"blah"`
}

func (m *mockFake) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	if m.logicalClock != nil {
		m.reconfiguredAt = m.logicalClock.Add(1)
	}
	if m.shouldRebuild {
		return resource.NewMustRebuildError(conf.ResourceName())
	}
	if c, err := resource.NativeConfig[*mockFakeConfig](conf); err == nil && m.failCount == 0 && c.ShouldFailReconfigure != 0 {
		m.failCount = c.ShouldFailReconfigure
	}
	if m.failCount != 0 {
		m.failCount--
		return errors.Errorf("failed to reconfigure (left %d)", m.failCount)
	}
	m.reconfCount++
	return nil
}

func (m *mockFake) Close(ctx context.Context) error {
	if m.logicalClock != nil {
		m.closedAt = m.logicalClock.Add(1)
	}
	m.closeCount++
	return nil
}

func (m *mockFakeConfig) Validate(path string) ([]string, error) {
	depOut := []string{}
	depOut = append(depOut, m.InferredDep...)
	return depOut, nil
}

type mockFake2 struct {
	resource.Named
	reconfCount int
	closeCount  int
}

func (m *mockFake2) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	m.reconfCount++
	return errors.New("oh no")
}

func (m *mockFake2) Close(ctx context.Context) error {
	m.closeCount++
	return nil
}
