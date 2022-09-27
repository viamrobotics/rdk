package robotimpl

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/a8m/envsubst"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/arm"
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
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	_ "go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/sensors"
	_ "go.viam.com/rdk/services/sensors/builtin"
	"go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/builtin"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

var serviceNames = resource.DefaultServices

func TestRobotReconfigure(t *testing.T) {
	test.That(t, len(resource.DefaultServices), test.ShouldEqual, 3)
	ConfigFromFile := func(t *testing.T, filePath string) *config.Config {
		t.Helper()
		logger := golog.NewTestLogger(t)
		buf, err := envsubst.ReadFile(filePath)
		test.That(t, err, test.ShouldBeNil)
		conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
		test.That(t, err, test.ShouldBeNil)
		return conf
	}
	mockSubtype := resource.NewSubtype(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.SubtypeName("mock"))
	mockNamed := func(name string) resource.Name {
		return resource.NameFromSubtype(mockSubtype, name)
	}
	modelName1 := utils.RandomAlphaString(5)
	modelName2 := utils.RandomAlphaString(5)
	test.That(t, os.Setenv("TEST_MODEL_NAME_1", modelName1), test.ShouldBeNil)
	test.That(t, os.Setenv("TEST_MODEL_NAME_2", modelName2), test.ShouldBeNil)

	registry.RegisterComponent(mockSubtype, modelName1, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			// test if implicit depencies are properly propagated
			for _, dep := range config.ConvertedAttributes.(*mockFakeConfig).InferredDep {
				if _, ok := deps[mockNamed(dep)]; !ok {
					return nil, errors.Errorf("inferred dependency %q cannot be found", mockNamed(dep))
				}
			}
			if config.ConvertedAttributes.(*mockFakeConfig).ShouldFail {
				return nil, errors.Errorf("cannot build %q for some obscure reason", config.Name)
			}
			return &mockFake{x: 5}, nil
		},
	})

	config.RegisterComponentAttributeMapConverter(
		mockSubtype.ResourceSubtype,
		modelName1,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf mockFakeConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&mockFakeConfig{})

	// these settings to be toggled in test cases specifically
	// testing for a reconfigurability mismatch
	reconfigurableTrue := true
	testReconfiguringMismatch := false
	registry.RegisterComponent(mockSubtype, modelName2, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			if reconfigurableTrue && testReconfiguringMismatch {
				reconfigurableTrue = false
				return &mockFake{x: 5}, nil
			}
			return &mockFake2{x: 5}, nil
		},
	})

	t.Run("no diff", func(t *testing.T) {
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
		test.That(t, rdktestutils.NewResourceNameSet(robot.ResourceNames()...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				baseNames,
				boardNames,
				mockNames,
				serviceNames,
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
				serviceNames,
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
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake2).reconfCount, test.ShouldEqual, 0)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("reconfiguring unreconfigurable", func(t *testing.T) {
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
				serviceNames,
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
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		reconfigurableTrue = false
		robot.Reconfigure(context.Background(), conf3)

		_, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldNotBeNil)

		reconfigurableTrue = true

		rr, ok := robot.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)

		rr.triggerConfig <- true

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
				serviceNames,
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
				serviceNames,
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
				serviceNames,
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

		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted[0:8]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
				[]resource.Name{mockNamed("mock1")},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[8:12]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
				[]resource.Name{mockNamed("mock2"), mockNamed("mock3")},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[12:14]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				baseNames,
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[14]), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
			)...))
	})

	t.Run("modificative deps diff", func(t *testing.T) {
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
				serviceNames,
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
				serviceNames,
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
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted[0:7]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[7:9]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				armNames,
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[9:11]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				baseNames,
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[11]), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
			)...))
	})

	t.Run("deletion deps diff", func(t *testing.T) {
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
				serviceNames,
			)...))
		test.That(t, utils.NewStringSet(robot.ProcessManager().ProcessIDs()...), test.ShouldResemble, utils.NewStringSet("1", "2"))

		robot.Reconfigure(context.Background(), conf4)
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
				serviceNames,
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
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				boardNames,
				serviceNames,
				[]resource.Name{
					resource.NameFromSubtype(unknownSubtype, "mock3"),
					resource.NameFromSubtype(unknownSubtype, "mock1"),
					mockNamed("mock6"),
				},
			)...))
	})

	t.Run("mixed deps diff", func(t *testing.T) {
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
				serviceNames,
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
				serviceNames,
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
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted[0:8]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
				[]resource.Name{arm.Named("arm1")},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[8:11]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{
					arm.Named("arm3"),
					base.Named("base1"),
					board.Named("board3"),
				},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[11:13]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{
					base.Named("base2"),
					board.Named("board2"),
				},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[13]), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{board.Named("board1")},
			)...))
	})

	t.Run("from empty conf with deps", func(t *testing.T) {
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
			rdktestutils.NewResourceNameSet(serviceNames...),
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
				serviceNames,
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
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted[0:8]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
				[]resource.Name{arm.Named("arm1")},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[8:11]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{
					arm.Named("arm3"),
					base.Named("base1"),
					board.Named("board3"),
				},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[11:13]...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{
					base.Named("base2"),
					board.Named("board2"),
				},
			)...))
		test.That(t, rdktestutils.NewResourceNameSet(sorted[13]), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				[]resource.Name{board.Named("board1")},
			)...))
	})

	t.Run("incremental deps config", func(t *testing.T) {
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
				serviceNames,
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
				serviceNames,
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
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock6, err := robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake2).x, test.ShouldEqual, 5)
		test.That(t, mock6.(*mockFake2).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
				boardNames,
				mockNames,
				encoderNames,
			)...))
	})

	t.Run("parent attribute change deps config", func(t *testing.T) {
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
				serviceNames,
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
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
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
				serviceNames,
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

		mock1, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock4, err = robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err = robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
	})

	t.Run("child component fails dep", func(t *testing.T) {
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
				serviceNames,
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
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock2, err := robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock3, err := robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock4, err := robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock5, err := robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock5.(*mockFake).reconfCount, test.ShouldEqual, 0)

		mock6, err := robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 0)

		_, ok = robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		sorted := robot.(*localRobot).manager.resources.TopologicalSort()
		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
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
				serviceNames,
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

		_, err = board.FromRobot(robot, "board2")
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldNotBeNil)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
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

		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
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
				serviceNames,
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

		mock1, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock1.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock2, err = robot.ResourceByName(mockNamed("mock2"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock2.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock2.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock3, err = robot.ResourceByName(mockNamed("mock3"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock3.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock3.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock4, err = robot.ResourceByName(mockNamed("mock4"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock4.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock4.(*mockFake).reconfCount, test.ShouldEqual, 1)

		mock5, err = robot.ResourceByName(mockNamed("mock5"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock5.(*mockFake).x, test.ShouldEqual, 5)
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

		rr.triggerConfig <- true

		utils.SelectContextOrWait(context.Background(), 200*time.Millisecond)
		mock6, err = robot.ResourceByName(mockNamed("mock6"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mock6.(*mockFake).x, test.ShouldEqual, 5)
		test.That(t, mock6.(*mockFake).reconfCount, test.ShouldEqual, 1)

		_, err = robot.ResourceByName(arm.Named("armFake"))
		test.That(t, err, test.ShouldBeNil)

		sorted = robot.(*localRobot).manager.resources.TopologicalSort()

		test.That(t, rdktestutils.NewResourceNameSet(sorted...), test.ShouldResemble, rdktestutils.NewResourceNameSet(
			rdktestutils.ConcatResourceNames(
				motorNames,
				serviceNames,
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
				serviceNames,
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
				serviceNames,
			)...))

		_, err = arm.FromRobot(robot, "arm1")
		test.That(t, err, test.ShouldNotBeNil)
		_, err = robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("test processes", func(t *testing.T) {
		logger := golog.NewTestLogger(t)
		tempDir := testutils.TempDirT(t, ".", "")
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

func TestSensorsServiceUpdate(t *testing.T) {
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

		svc, err := sensors.FromRobot(robot, resource.DefaultModelName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))
	})

	t.Run("two sensors to empty", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		svc, err := sensors.FromRobot(robot, resource.DefaultModelName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))

		robot.Reconfigure(context.Background(), emptyCfg)

		foundSensors, err = svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, foundSensors, test.ShouldBeEmpty)
	})

	t.Run("two sensors to two sensors", func(t *testing.T) {
		robot, err := New(context.Background(), cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, robot.Close(context.Background()), test.ShouldBeNil)
		}()

		svc, err := sensors.FromRobot(robot, resource.DefaultModelName)
		test.That(t, err, test.ShouldBeNil)

		foundSensors, err := svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))

		robot.Reconfigure(context.Background(), cfg)

		foundSensors, err = svc.Sensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rdktestutils.NewResourceNameSet(foundSensors...), test.ShouldResemble, rdktestutils.NewResourceNameSet(sensorNames...))
	})
}

func TestDefaultServiceReconfigure(t *testing.T) {
	logger := golog.NewTestLogger(t)

	visName := "vis"
	dmName := "dm"
	cfg1 := &config.Config{
		Services: []config.Service{
			{
				Name:      visName,
				Namespace: resource.ResourceNamespaceRDK,
				Type:      config.ServiceType(vision.SubtypeName),
				Model:     resource.DefaultModelName,
			},
			{
				Name:      dmName,
				Namespace: resource.ResourceNamespaceRDK,
				Type:      config.ServiceType(datamanager.SubtypeName),
				Model:     resource.DefaultModelName,
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
			vision.Named(visName),
			datamanager.Named(dmName),
			sensors.Named(resource.DefaultModelName),
		),
	)
	visName = "vis2"
	sName := "sensors"
	cfg2 := &config.Config{
		Services: []config.Service{
			{
				Name:      visName,
				Namespace: resource.ResourceNamespaceRDK,
				Type:      config.ServiceType(vision.SubtypeName),
				Model:     resource.DefaultModelName,
			},
			{
				Name:      sName,
				Namespace: resource.ResourceNamespaceRDK,
				Type:      config.ServiceType(sensors.SubtypeName),
				Model:     resource.DefaultModelName,
			},
		},
	}
	robot.Reconfigure(context.Background(), cfg2)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(robot.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			vision.Named(visName),
			datamanager.Named(resource.DefaultModelName),
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

	resourceNames := []resource.Name{movementsensor.Named("movement_sensor1"), movementsensor.Named("movement_sensor2")}
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
	t.Skip()
	loggerR := golog.NewDevelopmentLogger("remote")
	cfg, err := config.Read(context.Background(), "data/fake.json", loggerR)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()

	remote1, err := New(ctx, cfg, loggerR)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, remote1.Close(context.Background()), test.ShouldBeNil)
	}()

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err = remote1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	remote2, err := New(ctx, cfg, loggerR)
	test.That(t, err, test.ShouldBeNil)

	options, listener2, addr2 := robottestutils.CreateBaseOptionsAndListener(t)

	localConfig := &config.Config{
		Components: []config.Component{
			{
				Name:      "arm1",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
				DependsOn: []string{"foo:pieceGripper"},
			},
			{
				Name:      "arm2",
				Model:     "fake",
				Namespace: resource.ResourceNamespaceRDK,
				Type:      arm.SubtypeName,
				DependsOn: []string{"bar:pieceArm"},
			},
		},
		Services: []config.Service{},
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
	logger := golog.NewDevelopmentLogger("local")
	r, err := New(ctx, localConfig, logger)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), r), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			vision.Named(resource.DefaultModelName), sensors.Named(resource.DefaultModelName), datamanager.Named(resource.DefaultModelName),
			arm.Named("arm1"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			vision.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
		),
	)
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	rr, ok := r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- true

	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			vision.Named(resource.DefaultModelName), sensors.Named(resource.DefaultModelName), datamanager.Named(resource.DefaultModelName),
			arm.Named("arm1"), arm.Named("arm2"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			vision.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
			arm.Named("bar:pieceArm"),
			audioinput.Named("bar:mic1"),
			camera.Named("bar:cameraOver"),
			movementsensor.Named("bar:movement_sensor1"),
			movementsensor.Named("bar:movement_sensor2"),
			gripper.Named("bar:pieceGripper"),
			vision.Named("bar:builtin"),
			sensors.Named("bar:builtin"),
			datamanager.Named("bar:builtin"),
		),
	)

	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	utils.SelectContextOrWait(ctx, 15*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			vision.Named(resource.DefaultModelName), sensors.Named(resource.DefaultModelName), datamanager.Named(resource.DefaultModelName),
			arm.Named("arm1"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			vision.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
		),
	)

	remote3, err := New(ctx, cfg, loggerR)
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

	utils.SelectContextOrWait(ctx, 26*time.Second)

	rr, ok = r.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- true

	utils.SelectContextOrWait(ctx, 2*time.Second)

	test.That(
		t,
		rdktestutils.NewResourceNameSet(r.ResourceNames()...),
		test.ShouldResemble,
		rdktestutils.NewResourceNameSet(
			vision.Named(resource.DefaultModelName), sensors.Named(resource.DefaultModelName), datamanager.Named(resource.DefaultModelName),
			arm.Named("arm1"), arm.Named("arm2"),
			arm.Named("foo:pieceArm"),
			audioinput.Named("foo:mic1"),
			camera.Named("foo:cameraOver"),
			movementsensor.Named("foo:movement_sensor1"),
			movementsensor.Named("foo:movement_sensor2"),
			gripper.Named("foo:pieceGripper"),
			vision.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
			datamanager.Named("foo:builtin"),
			arm.Named("bar:pieceArm"),
			audioinput.Named("bar:mic1"),
			camera.Named("bar:cameraOver"),
			movementsensor.Named("bar:movement_sensor1"),
			movementsensor.Named("bar:movement_sensor2"),
			gripper.Named("bar:pieceGripper"),
			vision.Named("bar:builtin"),
			sensors.Named("bar:builtin"),
			datamanager.Named("bar:builtin"),
		),
	)
}

type mockFake struct {
	x           int
	reconfCount int
}

type mockFakeConfig struct {
	InferredDep []string `json:"inferred_dep"`
	ShouldFail  bool     `json:"should_fail"`
	Blah        int      `json:"blah"`
}

func (m *mockFake) Reconfigure(ctx context.Context, newResource resource.Reconfigurable) error {
	res, ok := newResource.(*mockFake)
	if !ok {
		return errors.Errorf("expected new mock to be %T but got %T", m, newResource)
	}
	m.x = res.x
	m.reconfCount++
	return nil
}

func (m *mockFake) UpdateAction(cfg *config.Component) config.UpdateActionType {
	return config.Reconfigure
}

func (m *mockFakeConfig) Validate(path string) ([]string, error) {
	depOut := []string{}
	depOut = append(depOut, m.InferredDep...)
	return depOut, nil
}

type mockFake2 struct {
	x           int
	reconfCount int
}
