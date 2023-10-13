//go:build !no_media

package robotimpl

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/a8m/envsubst"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
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

func TestRobotReconfigureMedia(t *testing.T) {
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

	defer func() {
		resource.Deregister(mockAPI, resource.DefaultModelFamily.WithModel(modelName1))
		resource.Deregister(mockAPI, resource.DefaultModelFamily.WithModel(modelName2))
	}()

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

		testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 20, func(tb testing.TB) {
			_, err = robot.ResourceByName(mockNamed("mock2"))
			test.That(tb, err, test.ShouldBeNil)
		})
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
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		test.That(tb, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)
	})
	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 600, func(tb testing.TB) {
		test.That(
			tb,
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
	})

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

	rr, ok = r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		test.That(tb, rdktestutils.NewResourceNameSet(r.ResourceNames()...), test.ShouldResemble, expectedSet)
	})
}
