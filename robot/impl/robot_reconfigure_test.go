package robotimpl

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a8m/envsubst"
	"github.com/golang/geo/r3"
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
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/sensors"
	_ "go.viam.com/rdk/services/sensors/builtin"
	"go.viam.com/rdk/spatialmath"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

var (
	// these settings to be toggled in test cases specifically
	// testing for a reconfigurability mismatch.
	reconfigurableTrue        = true
	testReconfiguringMismatch = false
)

func ConfigFromFile(tb testing.TB, filePath string) *config.Config {
	tb.Helper()
	logger := logging.NewTestLogger(tb)
	buf, err := envsubst.ReadFile(filePath)
	test.That(tb, err, test.ShouldBeNil)
	conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
	test.That(tb, err, test.ShouldBeNil)
	return conf
}

func ProcessConfig(tb testing.TB, conf *config.Config) *config.Config {
	tb.Helper()

	logger := logging.NewTestLogger(tb)
	processed, err := config.ProcessConfigLocalConfig(conf, logger)
	test.That(tb, err, test.ShouldBeNil)
	return processed
}

func TestRobotReconfigure(t *testing.T) {
	test.That(t, len(resource.DefaultServices()), test.ShouldEqual, 2)
	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	modelName2 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)
	model2 := resource.DefaultModelFamily.WithModel(modelName2)
	fakeModel = resource.DefaultModelFamily.WithModel("fake")

	resource.RegisterComponent(mockAPI, model1,
		resource.Registration[resource.Resource, *mockFakeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
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
	resource.RegisterComponent(mockAPI, model2,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				if reconfigurableTrue && testReconfiguringMismatch {
					reconfigurableTrue = false
					return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
				}
				return &mockFake2{Named: conf.ResourceName().AsNamed()}, nil
			},
		})

	defer func() {
		resource.Deregister(mockAPI, model1)
		resource.Deregister(mockAPI, model2)
	}()

	t.Run("no diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)

		// conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf1 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
				{
					Name:  "base1",
					API:   base.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model2,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})

		ctx := context.Background()
		robot := setupLocalRobot(t, ctx, conf1, logger)

		resources := robot.ResourceNames()
		test.That(t, len(resources), test.ShouldEqual, 7)

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}

		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		robot.Reconfigure(ctx, conf1)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := arm.FromRobot(robot, "arm1")
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
		logger := logging.NewTestLogger(t)
		// conf1 := ConfigFromFile(t, "data/diff_config_1.json")
		conf1 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
				{
					Name:  "base1",
					API:   base.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model2,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf3 := ConfigFromFile(t, "data/diff_config_4_bad.json")
		conf3 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
				{
					Name:  "base1",
					API:   base.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model2,
					Attributes: rutils.AttributeMap{
						"one": "2",
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf1, logger)

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		logger := logging.NewTestLogger(t)
		// conf1 := ConfigFromFile(t, "data/diff_config_deps1.json")
		conf1 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"inferred_dep": []string{
							"mock2",
							"mock3",
						},
					},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock3",
					API:   mockAPI,
					Model: model1,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf2 := ConfigFromFile(t, "data/diff_config_deps10.json")
		conf2 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:  "arm2",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"pwm_freq": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"inferred_dep": []string{
							"mock2",
							"mock3",
						},
					},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock3",
					API:   mockAPI,
					Model: model1,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf1, logger)

		armNames := []resource.Name{arm.Named("arm1")}
		baseNames := []resource.Name{base.Named("base1")}
		boardNames := []resource.Name{board.Named("board1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"),
		}

		robot.Reconfigure(context.Background(), conf1)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, motor.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			resource.DefaultServices(),
			mockNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		armNames = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames = []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := arm.FromRobot(robot, "arm1")
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
		logger := logging.NewTestLogger(t)
		// conf3 := ConfigFromFile(t, "data/diff_config_deps3.json")
		conf3 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
				{
					Name:  "arm2",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: fakeModel,
				},
				{
					Name:  "m2",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"pwm_freq": 4000,
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:  "m3",
					API:   motor.API,
					Model: fakeModel,
				},
				{
					Name:  "m4",
					API:   motor.API,
					Model: fakeModel,
				},
				{
					Name:  "base1",
					API:   base.API,
					Model: fakeModel,
				},
				{
					Name:  "base2",
					API:   base.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		conf2 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:  "arm2",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"pwm_freq": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf3, logger)

		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf3)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("5")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 4000)
		_, err = b.DigitalInterruptByName("encoder")
		test.That(t, err, test.ShouldNotBeNil)

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		_, err = b.DigitalInterruptByName("encoder")
		test.That(t, err, test.ShouldBeNil)

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
		logger := logging.NewTestLogger(t)
		// conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		conf2 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:  "arm2",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"pwm_freq": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})

		// conf4 := ConfigFromFile(t, "data/diff_config_deps4.json")
		conf4 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model2,
					// TODO: why doesn't this config break without dependencies?
					DependsOn: []string{"mock1", "mock3"},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf2, logger)

		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		arm2, err := arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldBeNil)

		test.That(t, arm2.(*fake.Arm).CloseCount, test.ShouldEqual, 0)
		robot.Reconfigure(context.Background(), conf4)
		test.That(t, arm2.(*fake.Arm).CloseCount, test.ShouldEqual, 1)

		boardNames = []resource.Name{board.Named("board1"), board.Named("board2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, motor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			[]resource.Name{
				mockNamed("mock6"),
			},
		))
	})

	t.Run("mixed deps diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		// conf2 := ConfigFromFile(t, "data/diff_config_deps2.json")
		conf2 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:  "arm2",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"pwm_freq": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "0",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
						},
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})

		// conf6 := ConfigFromFile(t, "data/diff_config_deps6.json")
		conf6 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:  "arm3",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m2",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"base1"},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"pwm_freq": 4000,
					},
				},
				{
					Name:  "m4",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"blab": "blob",
					},
					DependsOn: []string{"board3"},
				},
				{
					Name:  "m5",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"pwm_freq": 4000,
					},
					DependsOn: []string{"arm3", "board1"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board2"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "4",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoderC",
								"pin":  "22",
							},
						},
					},
				},
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board3",
					API:   board.API,
					Model: fakeModel,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf2, logger)

		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m3"), motor.Named("m4")}
		boardNames := []resource.Name{board.Named("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})
		b, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)
		pin, err := b.GPIOPinByName("1")
		test.That(t, err, test.ShouldBeNil)
		pwmF, err := pin.PWMFreq(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pwmF, test.ShouldEqual, 1000)
		_, err = b.DigitalInterruptByName("encoder")
		test.That(t, err, test.ShouldBeNil)

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
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		_, err = b.DigitalInterruptByName("encoder")
		test.That(t, err, test.ShouldNotBeNil)
		_, err = b.DigitalInterruptByName("encoderC")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board3")
		test.That(t, err, test.ShouldBeNil)

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
		logger := logging.NewTestLogger(t)
		// cempty := ConfigFromFile(t, "data/diff_config_empty.json")
		cempty := &config.Config{}
		// conf6 := ConfigFromFile(t, "data/diff_config_deps6.json")
		conf6 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:  "arm3",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m2",
					API:       motor.API,
					Model:     fakeModel,
					DependsOn: []string{"base1"},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"pwm_freq": 4000,
					},
				},
				{
					Name:  "m4",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"blab": "blob",
					},
					DependsOn: []string{"board3"},
				},
				{
					Name:  "m5",
					API:   motor.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"pwm_freq": 4000,
					},
					DependsOn: []string{"arm3", "board1"},
				},
				{
					Name:      "base1",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board2"},
				},
				{
					Name:      "base2",
					API:       base.API,
					Model:     fakeModel,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"analogs": []interface{}{
							map[string]interface{}{
								"name": "analog1",
								"pin":  "4",
							},
						},
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoderC",
								"pin":  "22",
							},
						},
					},
				},
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board3",
					API:   board.API,
					Model: fakeModel,
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})

		ctx := context.Background()
		robot := setupLocalRobot(t, ctx, cempty, logger)

		resources := robot.ResourceNames()
		test.That(t, len(resources), test.ShouldEqual, 2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, board.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), resource.DefaultServices())
		test.That(t, robot.ProcessManager().ProcessIDs(), test.ShouldBeEmpty)

		armNames := []resource.Name{arm.Named("arm1"), arm.Named("arm3")}
		baseNames := []resource.Name{base.Named("base1"), base.Named("base2")}
		motorNames := []resource.Name{motor.Named("m1"), motor.Named("m2"), motor.Named("m4"), motor.Named("m5")}
		boardNames := []resource.Name{
			board.Named("board1"),
			board.Named("board2"), board.Named("board3"),
		}
		robot.Reconfigure(context.Background(), conf6)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, base.NamesFromRobot(robot), rdktestutils.ExtractNames(baseNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := arm.FromRobot(robot, "arm1")
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
		_, err = b.DigitalInterruptByName("encoder")
		test.That(t, err, test.ShouldNotBeNil)
		_, err = b.DigitalInterruptByName("encoderC")
		test.That(t, err, test.ShouldBeNil)

		_, err = board.FromRobot(robot, "board3")
		test.That(t, err, test.ShouldBeNil)

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
		logger := logging.NewTestLogger(t)
		// conf4 := ConfigFromFile(t, "data/diff_config_deps4.json")
		conf4 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model2,
					// TODO: why doesn't this config break without dependencies?
					DependsOn: []string{"mock1", "mock3"},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		conf7 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "15",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "mock1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock4"},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock1"},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"one": "2",
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf4, logger)

		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, motor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := arm.FromRobot(robot, "arm1")
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
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

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
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
			motorNames,
			resource.DefaultServices(),
			boardNames,
			mockNames,
			encoderNames,
		))
	})

	t.Run("parent attribute change deps config", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		// conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		conf7 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "15",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "mock1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock4"},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock1"},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"one": "2",
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf8 := ConfigFromFile(t, "data/diff_config_deps8.json")
		conf8 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "16",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "22",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf7, logger)

		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		motorNames := []resource.Name{motor.Named("m1")}
		encoderNames := []resource.Name{encoder.Named("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"), mockNamed("mock6"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			encoderNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		m, err := motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

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

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
			motorNames,
			resource.DefaultServices(),
			boardNames,
			mockNames,
			encoderNames,
		))
		robot.Reconfigure(context.Background(), conf8)
		mockNames = []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		test.That(t, base.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))
		test.That(t, camera.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, gripper.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, sensor.NamesFromRobot(robot), test.ShouldBeEmpty)
		test.That(t, servo.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = arm.FromRobot(robot, "arm2")
		test.That(t, err, test.ShouldNotBeNil)

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		t.Log("the underlying pins changed but not the encoder names, so we keep the value")
		test.That(t, c, test.ShouldEqual, 0)

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

	// test starts with a working config, then reconfigures into a config where dependencies
	// fail to reconfigure, and then to a working config again.
	t.Run("child component fails dep", func(t *testing.T) {
		resetComponentFailureState()
		testReconfiguringMismatch = true
		reconfigurableTrue = true
		logger := logging.NewTestLogger(t)
		// conf7 := ConfigFromFile(t, "data/diff_config_deps7.json")
		conf7 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "14",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "15",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "mock1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock4"},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock1"},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"one": "2",
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		// conf9 := ConfigFromFile(t, "data/diff_config_deps9_bad.json")
		conf9 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "16",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "22",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "armFake",
					API:       arm.API,
					Model:     fakeModel,
					DependsOn: []string{"mock5", "mock6"},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
					DependsOn: []string{"mock4"},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
					Attributes: rutils.AttributeMap{
						"should_fail_reconfigure": 1,
					},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock1"},
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"one":                     6,
						"should_fail_reconfigure": 2,
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf7, logger)

		boardNames := []resource.Name{board.Named("board1"), board.Named("board2")}
		motorNames := []resource.Name{motor.Named("m1")}
		encoderNames := []resource.Name{encoder.Named("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
			mockNamed("mock6"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		m, err := motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err := m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

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

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
			motorNames,
			resource.DefaultServices(),
			boardNames,
			mockNames,
			encoderNames,
		))

		reconfigurableTrue = false
		robot.Reconfigure(context.Background(), conf9)

		mockNames = []resource.Name{
			mockNamed("mock2"),
			mockNamed("mock3"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

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

		// `mock6` is configured to be in a "failing" state.
		_, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldNotBeNil)

		// `armFake` depends on `mock6` and is therefore also in an error state.
		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldNotBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted = robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
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
		))

		// This configuration will put `mock6` into a good state after two calls to "reconfigure".
		// conf9good := ConfigFromFile(t, "data/diff_config_deps9_good.json")
		conf9good := ProcessConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   board.API,
					Model: fakeModel,
				},
				{
					Name:  "board1",
					API:   board.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"digital_interrupts": []interface{}{
							map[string]interface{}{
								"name": "encoder",
								"pin":  "16",
							},
							map[string]interface{}{
								"name": "encoder-b",
								"pin":  "22",
							},
						},
					},
				},
				{
					Name:  "m1",
					API:   motor.API,
					Model: resource.DefaultModelFamily.WithModel("gpio"),
					Attributes: rutils.AttributeMap{
						"board":   "board1",
						"encoder": "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"pwm_freq":           4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   encoder.API,
					Model: resource.DefaultModelFamily.WithModel("incremental"),
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "armFake",
					API:       arm.API,
					Model:     fakeModel,
					DependsOn: []string{"mock5", "mock6"},
				},
				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
					DependsOn: []string{"mock4"},
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:      "mock3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock2"},
					Attributes: rutils.AttributeMap{
						"blah": 10,
					},
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock3"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock1"},
					Attributes: rutils.AttributeMap{
						"blo": 10,
					},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"one": 6,
					},
				},
			},
			Processes: []pexec.ProcessConfig{
				{
					ID:      "1",
					Name:    "echo",
					Args:    []string{"hello", "world"},
					OneShot: true,
				},
				{
					ID:      "2",
					Name:    "echo",
					Args:    []string{"hello", "world", "again"},
					OneShot: true,
				},
			},
		})
		robot.Reconfigure(context.Background(), conf9good)

		mockNames = []resource.Name{
			mockNamed("mock2"), mockNamed("mock1"), mockNamed("mock3"),
			mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameElements(t, motor.NamesFromRobot(robot), rdktestutils.ExtractNames(motorNames...))
		rdktestutils.VerifySameElements(t, board.NamesFromRobot(robot), rdktestutils.ExtractNames(boardNames...))
		rdktestutils.VerifySameElements(t, encoder.NamesFromRobot(robot), rdktestutils.ExtractNames(encoderNames...))

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = board.FromRobot(robot, "board1")
		test.That(t, err, test.ShouldBeNil)

		m, err = motor.FromRobot(robot, "m1")
		test.That(t, err, test.ShouldBeNil)
		c, err = m.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c, test.ShouldEqual, 0)

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		// resources which failed previous reconfiguration attempts because of missing dependencies will be rebuilt,
		// so reconfCount should be 0. resources which failed previous reconfiguration attempts because of an error
		// during reconfiguration would not have its reconfCount reset, so reconfCount for mock4 should be 1.
		mock1, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

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
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		// `mock6` is configured to be in a "failing" state.
		_, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldNotBeNil)

		// `armFake` depends on `mock6` and is therefore also in an error state.
		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldNotBeNil)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)

		reconfigurableTrue = true

		rr, ok := robot.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)

		// The newly set configuration fixes the `mock6` component. A (second) reconfig should pick
		// that up and consequently bubble up the working `mock6` change to anything that depended
		// on `mock6`, notably `armFake`.
		rr.triggerConfig <- struct{}{}

		testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 30, func(tb testing.TB) {
			armFake, err := robot.ResourceByName(arm.Named("armFake"))
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, armFake, test.ShouldNotBeNil)
		})

		// Seeing `armFake` in a working state implies that `mock6` must also be in a working state
		// with its `reconfCount` bumped.
		mock6, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 1)

		sorted = robot.(*localRobot).manager.resources.TopologicalSort()
		sorted = rdktestutils.SubtractNames(sorted, robot.(*localRobot).manager.internalResourceNames()...)
		rdktestutils.VerifySameResourceNames(t, sorted, rdktestutils.ConcatResourceNames(
			motorNames,
			resource.DefaultServices(),
			boardNames,
			mockNames,
			encoderNames,
			[]resource.Name{
				arm.Named("armFake"),
				mockNamed("mock6"),
			},
		))
	})
	t.Run("complex diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		// conf1 := ConfigFromFile(t, "data/diff_config_deps11.json")
		conf1 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{

				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"inferred_dep": []string{
							"mock2",
							"mock3",
						},
					},
				},
				{
					Name:  "mock3",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model2,
					DependsOn: []string{"mock7"},
				},
				{
					Name:      "mock5",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"mock6"},
				},
				{
					Name:  "mock6",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock7",
					API:   arm.API,
					Model: fakeModel,
					Attributes: rutils.AttributeMap{
						"model-path": "../../components/arm/fake/fake_model.json",
					},
				},
			},
		})
		// conf2 := ConfigFromFile(t, "data/diff_config_deps12.json")
		conf2 := ProcessConfig(t, &config.Config{
			Components: []resource.Config{

				{
					Name:  "mock1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"inferred_dep": []string{
							"mock2",
							"mock3",
						},
					},
				},
				{
					Name:  "mock3",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:      "mock4",
					API:       mockAPI,
					Model:     model2,
					DependsOn: []string{"mock7"},
				},
				{
					Name:  "mock5",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "mock2",
					API:   mockAPI,
					Model: model1,
				},
			},
		})
		robot := setupLocalRobot(t, context.Background(), conf1, logger)

		armNames := []resource.Name{arm.Named("mock7")}
		mockNames := []resource.Name{
			mockNamed("mock3"), mockNamed("mock4"),
			mockNamed("mock6"), mockNamed("mock5"),
		}

		robot.Reconfigure(context.Background(), conf1)
		rdktestutils.VerifySameElements(t, arm.NamesFromRobot(robot), rdktestutils.ExtractNames(armNames...))
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			resource.DefaultServices(),
			mockNames,
		))
		_, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldNotBeNil)
		_, err = arm.FromRobot(robot, "mock7")
		test.That(t, err, test.ShouldBeNil)

		robot.Reconfigure(context.Background(), conf2)
		mockNames = []resource.Name{
			mockNamed("mock1"),
			mockNamed("mock3"), mockNamed("mock2"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

		test.That(t, arm.NamesFromRobot(robot), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			mockNames,
			resource.DefaultServices(),
		))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)
		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("test processes", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		tempDir := t.TempDir()
		robot := setupLocalRobot(t, context.Background(), &config.Config{}, logger)

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
		testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 50, func(tb testing.TB) {
			_, err = os.Stat(filepath.Join(tempDir, "afile"))
			test.That(tb, err, test.ShouldBeNil)
		})
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
					ID:   "filehandle2", // this transfer originF to target2F after 0.4s
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
		time.Sleep(3 * time.Second)
		_, err = targetF.Seek(0, 0)
		test.That(t, err, test.ShouldBeNil)
		n, err = targetF.Read(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, 128)
		test.That(t, r, test.ShouldResemble, token)
		time.Sleep(3 * time.Second)
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
		_, err = os.Stat(filepath.Join(tempDir, "afile"))
		test.That(t, err, test.ShouldNotBeNil)
	})
}

// this serves as a test for updateWeakDependents as the sensors service defines a weak
// dependency.
func TestSensorsServiceReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// emptyCfg, err := config.Read(context.Background(), "data/diff_config_empty.json", logger)
	emptyCfg := &config.Config{}
	// cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	// test.That(t, err, test.ShouldBeNil)
	cfg := ProcessConfig(t, &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceGripper",
				API:   gripper.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "mic1",
				API:   audioinput.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "camera",
				API:   camera.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "world",
					Translation: r3.Vector{
						X: 2000,
						Y: 500,
						Z: 1300,
					},
					Orientation: &spatialmath.OrientationConfig{
						Type: spatialmath.OrientationVectorDegreesType,
						Value: map[string]any{
							"x":  0,
							"y":  0,
							"z":  1,
							"th": 180,
						},
					},
				},
			},
			{
				Name:  "pieceArm",
				API:   arm.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "world",
					Translation: r3.Vector{
						X: 500,
						Y: 500,
						Z: 1000,
					},
				},
				Attributes: rutils.AttributeMap{
					"model-path": "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "movement_sensor1",
				API:   movementsensor.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "movement_sensor2",
				API:   movementsensor.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "pieceArm",
				},
				Attributes: rutils.AttributeMap{
					"relative": true,
				},
			},
		},
	})

	sensorNames := []resource.Name{movementsensor.Named("movement_sensor1"), movementsensor.Named("movement_sensor2")}

	t.Run("empty to two sensors", func(t *testing.T) {
		robot := setupLocalRobot(t, context.Background(), emptyCfg, logger)

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		rdktestutils.VerifySameResourceNames(t, foundSensors, sensorNames)
	})

	t.Run("two sensors to empty", func(t *testing.T) {
		robot := setupLocalRobot(t, context.Background(), cfg, logger)

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		rdktestutils.VerifySameResourceNames(t, foundSensors, sensorNames)

		robot.Reconfigure(context.Background(), emptyCfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)
	})

	t.Run("two sensors to two sensors", func(t *testing.T) {
		robot := setupLocalRobot(t, context.Background(), cfg, logger)

		svc, err := sensors.FromRobot(robot, resource.DefaultServiceName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		rdktestutils.VerifySameResourceNames(t, foundSensors, sensorNames)

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		rdktestutils.VerifySameResourceNames(t, foundSensors, sensorNames)
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
	logger := logging.NewTestLogger(t)

	var emptyCfg config.Config
	test.That(t, emptyCfg.Ensure(false, logger), test.ShouldBeNil)

	robot := setupLocalRobot(t, context.Background(), &emptyCfg, logger)

	// Register a `Resource` that generates weak dependencies. Specifically instance of
	// this resource will depend on every `component` resource. See the definition of
	// `internal.ComponentDependencyWildcardMatcher`.
	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weaktype")
	weakModel := resource.NewModel(uuid.NewString(), "soweak", "weak1000")
	weak1Name := resource.NewName(weakAPI, "weak1")
	resource.Register(
		weakAPI,
		weakModel,
		resource.Registration[*someTypeWithWeakAndStrongDeps, *someTypeWithWeakAndStrongDepsConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (*someTypeWithWeakAndStrongDeps, error) {
				return &someTypeWithWeakAndStrongDeps{
					Named:     conf.ResourceName().AsNamed(),
					resources: deps,
				}, nil
			},
			WeakDependencies: []resource.Matcher{resource.TypeMatcher{Type: resource.APITypeComponentName}},
		})
	defer func() {
		resource.Deregister(weakAPI, weakModel)
	}()

	// Create a configuration with a single component that has an explicit, unresolved
	// dependency. Reconfiguring will succeed, but getting a handle on the `weak1Name` resource fails
	// with `unresolved dependencies`.
	base1Name := base.Named("base1")
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

	_, err := robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldNotBeNil)
	// Assert that the explicit dependency was observed.
	test.That(t, err.Error(), test.ShouldContainSubstring, "unresolved dependencies")
	test.That(t, err.Error(), test.ShouldContainSubstring, "base1")

	// Reconfigure without the explicit dependency. While also adding a second component that would
	// have satisfied the dependency from the prior `weakCfg1`. Due to the weak dependency wildcard
	// matcher, this `base1` component will be parsed as a weak dependency of `weak1`.
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
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	// Assert that the weak dependency was tracked.
	test.That(t, weak1.resources, test.ShouldHaveLength, 1)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)

	// Reconfigure again with a new third `arm` component.
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
	// With two other components, `weak1` now has two (weak) dependencies.
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.resources, test.ShouldContainKey, arm1Name)

	base2Name := base.Named("base2")
	weakCfg5 := config.Config{
		Components: []resource.Config{
			{
				Name:  weak1Name.Name,
				API:   weakAPI,
				Model: weakModel,
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this `weak1`
				// component. We change the `Attributes` field from the previous (nil) value to
				// accomplish that.
				Attributes: rutils.AttributeMap{"version": 1},
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
				Name:       weak1Name.Name,
				API:        weakAPI,
				Model:      weakModel,
				Attributes: rutils.AttributeMap{"version": 2},
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
				Name:       weak1Name.Name,
				API:        weakAPI,
				Model:      weakModel,
				Attributes: rutils.AttributeMap{"version": 3},
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
	logger := logging.NewTestLogger(t)

	motionName := "motion"
	cfg1 := &config.Config{
		Services: []resource.Config{
			{
				Name:  motionName,
				API:   motion.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}
	robot := setupLocalRobot(t, context.Background(), cfg1, logger)

	rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(),
		[]resource.Name{
			motion.Named(motionName),
			sensors.Named(resource.DefaultServiceName),
		},
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
	rdktestutils.VerifySameResourceNames(
		t,
		robot.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(sName),
		},
	)
}

func TestStatusServiceUpdate(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// emptyCfg, err := config.Read(context.Background(), "data/diff_config_empty.json", logger)
	// test.That(t, err, test.ShouldBeNil)
	emptyCfg := &config.Config{}
	// cfg, cfgErr := config.Read(context.Background(), "data/fake.json", logger)
	// test.That(t, cfgErr, test.ShouldBeNil)
	cfg := ProcessConfig(t, &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceGripper",
				API:   gripper.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "mic1",
				API:   audioinput.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "camera",
				API:   camera.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "world",
					Translation: r3.Vector{
						X: 2000,
						Y: 500,
						Z: 1300,
					},
					Orientation: &spatialmath.OrientationConfig{
						Type: spatialmath.OrientationVectorDegreesType,
						Value: map[string]any{
							"x":  0,
							"y":  0,
							"z":  1,
							"th": 180,
						},
					},
				},
			},
			{
				Name:  "pieceArm",
				API:   arm.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "world",
					Translation: r3.Vector{
						X: 500,
						Y: 500,
						Z: 1000,
					},
				},
				Attributes: rutils.AttributeMap{
					"model-path": "../../components/arm/fake/fake_model.json",
				},
			},
			{
				Name:  "movement_sensor1",
				API:   arm.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
			{
				Name:  "movement_sensor2",
				API:   arm.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "pieceArm",
				},
				Attributes: rutils.AttributeMap{
					"relative": true,
				},
			},
		},
	})

	resourceNames := []resource.Name{
		movementsensor.Named("movement_sensor1"),
		movementsensor.Named("movement_sensor2"),
	}
	expected := map[resource.Name]interface{}{
		movementsensor.Named("movement_sensor1"): map[string]interface{}{},
		movementsensor.Named("movement_sensor2"): map[string]interface{}{},
	}

	t.Run("empty to not empty", func(t *testing.T) {
		robot := setupLocalRobot(t, context.Background(), emptyCfg, logger)

		_, err := robot.Status(context.Background(), resourceNames)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		robot.Reconfigure(context.Background(), cfg)

		statuses, err := robot.Status(context.Background(), resourceNames)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(statuses), test.ShouldEqual, 2)
		test.That(t, statuses[0].Status, test.ShouldResemble, expected[statuses[0].Name])
		test.That(t, statuses[1].Status, test.ShouldResemble, expected[statuses[1].Name])
	})

	t.Run("not empty to empty", func(t *testing.T) {
		robot := setupLocalRobot(t, context.Background(), cfg, logger)

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
		robot := setupLocalRobot(t, context.Background(), cfg, logger)

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
	// This tests that a main part is able to start up with an offline remote robot, connect to it and
	// depend on the remote robot's resources when it comes online. And react appropriately when the remote robot goes offline again.

	// If a new robot object/process comes online at the same address+port, the main robot should still be able
	// to use the new remote robot's resources.

	// To do so, the test initially sets up two remote robots, Remote 1 and 2, and then a third remote, Remote 3,
	// in the following scenario:
	// 1) Remote 1's server is started.
	// 2) The main robot is then set up with resources that depend on resources on both Remote 1 and 2. Since
	//    Remote 2 is not up, their resources are not available to the main robot.
	// 3) After initial configuration, Remote 2's server starts up and the main robot should then connect
	//	  and pick up the new available resources.
	// 4) Remote 2 goes down, and the main robot should remove any resources or resources that depend on
	//    resources from Remote 2.
	// 5) Remote 3 comes online at the same address as Remote 2, and the main robot should treat it the same as
	//    if Remote 2 came online again and re-add all the removed resources.
	logger := logging.NewTestLogger(t)
	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "remoteArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	// set up and start remote1's web service
	remote1 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote1"))
	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := remote1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// set up but do not start remote2's web service
	remote2 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote2"))
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
				DependsOn: []string{"foo:remoteArm"},
			},
			{
				Name:  "arm2",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"bar:remoteArm"},
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
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("main"))

	// assert all of remote1's resources exist on main but none of remote2's
	rdktestutils.VerifySameResourceNames(
		t,
		r.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			arm.Named("arm1"),
			arm.Named("foo:remoteArm"),
			motion.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
		},
	)

	// start remote2's web service
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	mainPartAndFooAndBarResources := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("arm2"),
		arm.Named("foo:remoteArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:remoteArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), mainPartAndFooAndBarResources)
	})
	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
				arm.Named("arm1"),
				arm.Named("foo:remoteArm"),
				motion.Named("foo:builtin"),
				sensors.Named("foo:builtin"),
			},
		)
	})

	remote3 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote3"))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener2, err = net.Listen("tcp", listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener2
	err = remote3.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), mainPartAndFooAndBarResources)
	})
}

func TestRemoteRobotsUpdate(t *testing.T) {
	// The test tests that the robot is able to update when multiple remote robot
	// updates happen at the same time.
	logger := logging.NewTestLogger(t)
	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API: arm.API,
			},
		},
	}
	ctx := context.Background()
	remote := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote"))

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := remote.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr1,
			},
			{
				Name:    "hello",
				Address: addr1,
			},
			{
				Name:    "world",
				Address: addr1,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:arm1"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:arm1"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		arm.Named("hello:arm1"),
		motion.Named("hello:builtin"),
		sensors.Named("hello:builtin"),
		arm.Named("world:arm1"),
		motion.Named("world:builtin"),
		sensors.Named("world:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, remote.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
		)
	})
}

func TestInferRemoteRobotDependencyConnectAtStartup(t *testing.T) {
	// The test tests that the robot is able to infer remote dependencies
	// if remote name is not part of the specified dependency
	// and the remote is online at start up.
	logger := logging.NewTestLogger(t)

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
	foo := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo"))

	options, listener1, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := foo.StartWeb(ctx, options)
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
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))
	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
	}

	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedSet)
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
		)
	})

	foo2 := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo2"))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener1, err = net.Listen("tcp", listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener1
	err = foo2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
}

func TestInferRemoteRobotDependencyConnectAfterStartup(t *testing.T) {
	// The test tests that the robot is able to infer remote dependencies
	// if remote name is not part of the specified dependency
	// and the remote is offline at start up.
	logger := logging.NewTestLogger(t)

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

	foo := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo"))

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
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))
	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
		},
	)
	err := foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
		)
	})
}

func TestInferRemoteRobotDependencyAmbiguous(t *testing.T) {
	// The test tests that the robot will not build a resource if the dependency
	// is ambiguous. In this case, "pieceArm" can refer to both "foo:pieceArm"
	// and "bar:pieceArm".
	logger := logging.NewTestLogger(t)

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

	foo := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("foo"))
	bar := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("bar"))

	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := foo.StartWeb(ctx, options1)
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
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
	}

	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedSet)

	// we expect the robot to correctly detect the ambiguous dependency and not build the resource
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 150, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})

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

	finalSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		arm.Named("arm1"),
	}

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), finalSet)
	})
}

func TestReconfigureModelRebuild(t *testing.T) {
	logger := logging.NewTestLogger(t)

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
			logger logging.Logger,
		) (resource.Resource, error) {
			return &mockFake{Named: conf.ResourceName().AsNamed(), shouldRebuild: true}, nil
		},
	})
	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

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

	r := setupLocalRobot(t, ctx, cfg, logger)

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
				Name:  "one",
				Model: model1,
				API:   mockAPI,
				// Change the `Attributes` to force this component to be reconfigured.
				Attributes:          rutils.AttributeMap{"version": 1},
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
	logger := logging.NewTestLogger(t)

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
			logger logging.Logger,
		) (resource.Resource, error) {
			return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
		},
	})
	resource.RegisterComponent(mockAPI, model2, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return &mockFake2{Named: conf.ResourceName().AsNamed()}, nil
		},
	})

	defer func() {
		resource.Deregister(mockAPI, model1)
		resource.Deregister(mockAPI, model2)
	}()

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

	r := setupLocalRobot(t, ctx, cfg, logger)

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

func TestReconfigureModelSwitchErr(t *testing.T) {
	logger := logging.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	mockNamed := func(name string) resource.Name {
		return resource.NewName(mockAPI, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	newCount := 0
	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			newCount++
			return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
		},
	})

	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

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

	r := setupLocalRobot(t, ctx, cfg, logger)
	test.That(t, newCount, test.ShouldEqual, 1)

	name1 := mockNamed("one")
	res1, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 0)

	modelName2 := utils.RandomAlphaString(5)
	model2 := resource.DefaultModelFamily.WithModel(modelName2)

	newCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model2,
				API:   mockAPI,
			},
		},
	}
	r.Reconfigure(ctx, newCfg)
	test.That(t, newCount, test.ShouldEqual, 1)

	_, err = r.ResourceByName(name1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 1)

	r.Reconfigure(ctx, cfg)
	test.That(t, newCount, test.ShouldEqual, 2)

	res2, err := r.ResourceByName(name1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res2, test.ShouldNotEqual, res1)
	test.That(t, res1.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res1.(*mockFake).closeCount, test.ShouldEqual, 1)
	test.That(t, res2.(*mockFake).reconfCount, test.ShouldEqual, 0)
	test.That(t, res2.(*mockFake).closeCount, test.ShouldEqual, 0)
}

func TestReconfigureRename(t *testing.T) {
	logger := logging.NewTestLogger(t)

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
			logger logging.Logger,
		) (resource.Resource, error) {
			return &mockFake{
				Named:        conf.ResourceName().AsNamed(),
				logicalClock: &logicalClock,
				createdAt:    int(logicalClock.Add(1)),
			}, nil
		},
	})
	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

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

	r := setupLocalRobot(t, ctx, cfg, logger)

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

// tests that the resource configuration timeout is passed into each resource constructor.
func TestResourceConstructTimeout(t *testing.T) {
	logger := logging.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	var timeout time.Duration

	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			deadline, ok := ctx.Deadline()
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, time.Now().Add(timeout), test.ShouldHappenOnOrAfter, deadline)
			return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
		},
	})
	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model1,
				API:   mockAPI,
			},
			{
				Name:  "two",
				Model: model1,
				API:   mockAPI,
			},
		},
	}
	t.Run("new", func(t *testing.T) {
		timeout = 50 * time.Millisecond
		test.That(t, os.Setenv(rutils.ResourceConfigurationTimeoutEnvVar, timeout.String()),
			test.ShouldBeNil)
		defer func() {
			test.That(t, os.Unsetenv(rutils.ResourceConfigurationTimeoutEnvVar),
				test.ShouldBeNil)
		}()

		r := setupLocalRobot(t, context.Background(), cfg, logger)
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	})
	t.Run("reconfigure", func(t *testing.T) {
		timeout = rutils.DefaultResourceConfigurationTimeout
		r := setupLocalRobot(t, context.Background(), cfg, logger)

		timeout = 200 * time.Millisecond
		test.That(t, os.Setenv(rutils.ResourceConfigurationTimeoutEnvVar, timeout.String()),
			test.ShouldBeNil)
		defer func() {
			test.That(t, os.Unsetenv(rutils.ResourceConfigurationTimeoutEnvVar),
				test.ShouldBeNil)
		}()

		newCfg := &config.Config{
			Components: []resource.Config{
				{
					Name:  "one",
					Model: model1,
					API:   mockAPI,
				},
				{
					Name:  "two",
					Model: model1,
					API:   mockAPI,
				},
				{
					Name:  "three",
					Model: model1,
					API:   mockAPI,
				},
			},
		}

		r.Reconfigure(context.Background(), newCfg)
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	})
}

// tests that on context cancellation, the resource re/configuration loop never gets inside the resource constructor.
func TestResourceConstructCtxCancel(t *testing.T) {
	logger := logging.NewTestLogger(t)

	contructCount := 0
	var wg sync.WaitGroup

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	type cancelFunc struct {
		c context.CancelFunc
	}
	var cFunc cancelFunc

	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			contructCount++
			wg.Add(1)
			defer wg.Done()
			cFunc.c()
			<-ctx.Done()
			return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
		},
	})
	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "one",
				Model: model1,
				API:   mockAPI,
			},
			{
				Name:  "two",
				Model: model1,
				API:   mockAPI,
				// we need "two" to depend on "one" to prevent a flaky test here. the
				// subtests below assert that only one resource gets configured after we
				// cancel. however, independent resources are constructed concurrently so
				// this assertion is not reliable, so we force it by adding a dependency
				// relationship.
				DependsOn: []string{"one"},
			},
		},
	}
	t.Run("new", func(t *testing.T) {
		contructCount = 0
		ctxWithCancel, cancel := context.WithCancel(context.Background())
		cFunc.c = cancel
		r := setupLocalRobot(t, ctxWithCancel, cfg, logger)
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)

		wg.Wait()
		test.That(t, contructCount, test.ShouldEqual, 1)
	})
	t.Run("reconfigure", func(t *testing.T) {
		contructCount = 0
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		test.That(t, contructCount, test.ShouldEqual, 0)

		ctxWithCancel, cancel := context.WithCancel(context.Background())
		cFunc.c = cancel
		r.Reconfigure(ctxWithCancel, cfg)
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)

		wg.Wait()
		test.That(t, contructCount, test.ShouldEqual, 1)
	})
}

func TestResourceCloseNoHang(t *testing.T) {
	logger := logging.NewTestLogger(t)

	mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	modelName1 := utils.RandomAlphaString(5)
	model1 := resource.DefaultModelFamily.WithModel(modelName1)

	mf := &mockFake{Named: resource.NewName(mockAPI, "mock").AsNamed()}
	resource.RegisterComponent(mockAPI, model1, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			return mf, nil
		},
	})
	defer func() {
		resource.Deregister(mockAPI, model1)
	}()

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "mock",
				Model: model1,
				API:   mockAPI,
			},
		},
	}
	r := setupLocalRobot(t, context.Background(), cfg, logger)

	test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	test.That(t, mf.closeCtxDeadline, test.ShouldNotBeNil)
	test.That(t, time.Now().Add(resourceCloseTimeout), test.ShouldHappenOnOrAfter, mf.closeCtxDeadline)
}

type mockFake struct {
	resource.Named
	createdAt        int
	reconfCount      int
	reconfiguredAt   int64
	failCount        int
	shouldRebuild    bool
	closedAt         int64
	closeCount       int
	closeCtxDeadline time.Time
	logicalClock     *atomic.Int64
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
	if dl, exists := ctx.Deadline(); exists {
		m.closeCtxDeadline = dl
	}
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
