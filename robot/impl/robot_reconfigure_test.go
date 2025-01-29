package robotimpl

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/a8m/envsubst"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	_ "go.viam.com/rdk/services/datamanager/builtin"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/services/motion"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	_ "go.viam.com/rdk/services/motion/builtin"
	rdktestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

var (
	// these settings to be toggled in test cases specifically
	// testing for a reconfigurability mismatch.
	reconfigurableTrue        = true
	testReconfiguringMismatch = false
)

var mockAPI = resource.APINamespaceRDK.WithComponentType("mock")

func mockNamed(name string) resource.Name {
	return resource.NewName(mockAPI, name)
}

func ConfigFromFile(tb testing.TB, filePath string) *config.Config {
	tb.Helper()
	logger := logging.NewTestLogger(tb)
	buf, err := envsubst.ReadFile(filePath)
	test.That(tb, err, test.ShouldBeNil)
	conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
	test.That(tb, err, test.ShouldBeNil)
	return conf
}

func processConfig(tb testing.TB, conf *config.Config) *config.Config {
	tb.Helper()

	logger := logging.NewTestLogger(tb)
	test.That(tb, conf.ProcessLocal(logger), test.ShouldBeNil)
	return conf
}

func registerMockComponent[R resource.Resource, CV resource.ConfigValidator](
	tb testing.TB,
	registration resource.Registration[R, CV],
) resource.Model {
	tb.Helper()

	modelName := utils.RandomAlphaString(5)
	model := resource.DefaultModelFamily.WithModel(modelName)

	resource.RegisterComponent(mockAPI, model, registration)
	tb.Cleanup(func() { resource.Deregister(mockAPI, model) })

	return model
}

func TestRobotReconfigure(t *testing.T) {
	test.That(t, len(resource.DefaultServices()), test.ShouldEqual, 1)

	model1 := registerMockComponent(
		t,
		resource.Registration[resource.Resource, *mockFakeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				// test if implicit depencies are properly propagated
				convAttrs := conf.ConvertedAttributes.(*mockFakeConfig)
				for _, dep := range convAttrs.InferredDep {
					if _, ok := deps[mockNamed(dep)]; !ok {
						return nil, errors.Errorf("inferred dependency %q cannot be found", mockNamed(dep))
					}
				}
				if convAttrs.ShouldFail {
					return nil, errors.Errorf("cannot build %q for some obscure reason", conf.Name)
				}
				return &mockFake{
					Named:       conf.ResourceName().AsNamed(),
					Value:       convAttrs.Value,
					childValues: make(map[string]int),
				}, nil
			},
		})

	resetComponentFailureState := func() {
		reconfigurableTrue = true
		testReconfiguringMismatch = false
	}

	model2 := registerMockComponent(
		t,
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

	mockWithDepModel := registerMockComponent(t, resource.Registration[resource.Resource, *mockWithDepConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			convAttrs := conf.ConvertedAttributes.(*mockWithDepConfig)
			mockDepName := convAttrs.MockDep
			mockDep, ok := deps[mockNamed(mockDepName)]
			if !ok {
				return nil, errors.New("missing dependency")
			}
			parent := mockDep.(*mockFake)
			slot := convAttrs.Slot
			value := convAttrs.Value
			parent.SetChildValue(slot, value)
			return &mockWithDep{
				Named:  conf.ResourceName().AsNamed(),
				Slot:   slot,
				Value:  value,
				parent: parent,
			}, nil
		},
	})

	t.Run("no diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)

		conf1 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "base1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
		test.That(t, len(resources), test.ShouldEqual, 6)

		armNames := []resource.Name{mockNamed("arm1")}
		baseNames := []resource.Name{mockNamed("base1")}
		boardNames := []resource.Name{mockNamed("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}

		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

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
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
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
		conf1 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "base1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
		conf3 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "base1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		armNames := []resource.Name{mockNamed("arm1")}
		baseNames := []resource.Name{mockNamed("base1")}
		boardNames := []resource.Name{mockNamed("board1")}
		mockNames := []resource.Name{mockNamed("mock1"), mockNamed("mock2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		arm1, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		base1, err := robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		board1, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		resource1, err := robot.ResourceByName(mockNamed("arm1"))
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
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		test.That(t, mock1.(*mockFake).reconfCount, test.ShouldEqual, 0)

		newArm1, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newArm1, test.ShouldEqual, arm1)

		newBase1, err := robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newBase1, test.ShouldEqual, base1)

		newBoard1, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, newBoard1, test.ShouldEqual, board1)

		newResource1, err := robot.ResourceByName(mockNamed("arm1"))
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
		conf1 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
		conf2 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:      "arm2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"slot":  "1",
						"value": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		armNames := []resource.Name{mockNamed("arm1")}
		baseNames := []resource.Name{mockNamed("base1")}
		boardNames := []resource.Name{mockNamed("board1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"),
		}

		robot.Reconfigure(context.Background(), conf1)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			resource.DefaultServices(),
			mockNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		armNames = []resource.Name{mockNamed("arm1"), mockNamed("arm2")}
		baseNames = []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames := []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m3"), mockNamed("m4")}
		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			mockNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldBeNil)

		b, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("1"), test.ShouldEqual, 1000)

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
		conf3 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "arm1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "arm2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "m1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"slot":  "5",
						"value": 4000,
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:  "m3",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "m4",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "base1",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "base2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
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
		//nolint:dupl
		conf2 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:      "arm2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"slot":  "1",
						"value": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		armNames := []resource.Name{mockNamed("arm1"), mockNamed("arm2")}
		baseNames := []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames := []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m3"), mockNamed("m4")}
		boardNames := []resource.Name{mockNamed("board1")}

		robot.Reconfigure(context.Background(), conf3)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		b, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("5"), test.ShouldEqual, 4000)

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldBeNil)

		b, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("5"), test.ShouldEqual, 0)
		test.That(t, b.(*mockFake).GetChildValue("1"), test.ShouldEqual, 1000)

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
		//nolint:dupl
		conf2 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:      "arm2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"slot":  "1",
						"value": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		conf4 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		armNames := []resource.Name{mockNamed("arm1"), mockNamed("arm2")}
		baseNames := []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames := []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m3"), mockNamed("m4")}
		boardNames := []resource.Name{mockNamed("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		arm2, err := robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldBeNil)

		test.That(t, arm2.(*mockFake).closeCount, test.ShouldEqual, 0)
		robot.Reconfigure(context.Background(), conf4)
		test.That(t, arm2.(*mockFake).closeCount, test.ShouldEqual, 1)

		boardNames = []resource.Name{mockNamed("board1"), mockNamed("board2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		//nolint:dupl
		conf2 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:      "arm2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "1",
						},
						"slot":  "1",
						"value": 1000,
					},
					DependsOn: []string{"arm2", "board1"},
				},
				{
					Name:      "m3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm1"},
				},
				{
					Name:      "m4",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"arm2"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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

		conf6 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "arm3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:  "m2",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
					},
					DependsOn: []string{"base1"},
				},
				{
					Name:  "m1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"value": 4000,
					},
				},
				{
					Name:  "m4",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blab": "blob",
					},
					DependsOn: []string{"board3"},
				},
				{
					Name:  "m5",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"board": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"value": 4000,
					},
					DependsOn: []string{"arm3", "board1"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board2"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board3",
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
		robot := setupLocalRobot(t, context.Background(), conf2, logger)

		armNames := []resource.Name{mockNamed("arm1"), mockNamed("arm2")}
		baseNames := []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames := []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m3"), mockNamed("m4")}
		boardNames := []resource.Name{mockNamed("board1")}

		robot.Reconfigure(context.Background(), conf2)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})
		b, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("1"), test.ShouldEqual, 1000)

		armNames = []resource.Name{mockNamed("arm1"), mockNamed("arm3")}
		baseNames = []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames = []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m4"), mockNamed("m5")}
		boardNames = []resource.Name{
			mockNamed("board1"),
			mockNamed("board2"), mockNamed("board3"),
		}

		motor2, err := robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldBeNil)

		robot.Reconfigure(context.Background(), conf6)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("arm3"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldBeNil)

		nextMotor2, err := robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldBeNil)
		// m2 lost its dependency on arm2 after looking conf6
		// but only relies on base1 so it should never have been
		// removed but only reconfigured.
		test.That(t, nextMotor2, test.ShouldPointTo, motor2)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m5"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldBeNil)

		b, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("1"), test.ShouldEqual, 0)

		_, err = robot.ResourceByName(mockNamed("board3"))
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
					[]resource.Name{mockNamed("arm1")},
				),
				{
					mockNamed("arm3"),
					mockNamed("base1"),
					mockNamed("board3"),
				},
				{
					mockNamed("base2"),
					mockNamed("board2"),
				},
				{mockNamed("board1")},
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("from empty conf with deps", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		cempty := &config.Config{}
		conf6 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:      "arm1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "arm3",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base2"},
				},
				{
					Name:      "m2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"base1"},
				},
				{
					Name:  "m1",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"value": 4000,
					},
				},
				{
					Name:  "m4",
					API:   mockAPI,
					Model: model1,
					Attributes: rutils.AttributeMap{
						"blab": "blob",
					},
					DependsOn: []string{"board3"},
				},
				{
					Name:  "m5",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"pwm": "5",
						},
						"slot":  "5",
						"value": 4000,
					},
					DependsOn: []string{"arm3", "board1"},
				},
				{
					Name:      "base1",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board2"},
				},
				{
					Name:      "base2",
					API:       mockAPI,
					Model:     model1,
					DependsOn: []string{"board1"},
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board3",
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

		ctx := context.Background()
		robot := setupLocalRobot(t, ctx, cempty, logger)

		resources := robot.ResourceNames()
		test.That(t, len(resources), test.ShouldEqual, 1)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), resource.DefaultServices())
		test.That(t, robot.ProcessManager().ProcessIDs(), test.ShouldBeEmpty)

		armNames := []resource.Name{mockNamed("arm1"), mockNamed("arm3")}
		baseNames := []resource.Name{mockNamed("base1"), mockNamed("base2")}
		motorNames := []resource.Name{mockNamed("m1"), mockNamed("m2"), mockNamed("m4"), mockNamed("m5")}
		boardNames := []resource.Name{
			mockNamed("board1"),
			mockNamed("board2"), mockNamed("board3"),
		}
		robot.Reconfigure(context.Background(), conf6)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			baseNames,
			boardNames,
			motorNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("arm3"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m5"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldBeNil)

		b, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b.(*mockFake).GetChildValue("5"), test.ShouldEqual, 4000)

		_, err = robot.ResourceByName(mockNamed("board3"))
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
					[]resource.Name{mockNamed("arm1")},
				),
				{
					mockNamed("arm3"),
					mockNamed("base1"),
					mockNamed("board3"),
				},
				{
					mockNamed("base2"),
					mockNamed("board2"),
				},
				{mockNamed("board1")},
			},
			robot.(*localRobot).manager.internalResourceNames()...,
		)
	})

	t.Run("incremental deps config", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		conf4 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
		conf7 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
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

		boardNames := []resource.Name{mockNamed("board1"), mockNamed("board2")}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
		test.That(t, err, test.ShouldBeNil)

		_, ok := robot.ProcessManager().ProcessByID("1")
		test.That(t, ok, test.ShouldBeTrue)
		_, ok = robot.ProcessManager().ProcessByID("2")
		test.That(t, ok, test.ShouldBeTrue)
		motorNames := []resource.Name{mockNamed("m1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
			mockNamed("mock6"),
		}
		encoderNames := []resource.Name{mockNamed("e1")}

		robot.Reconfigure(context.Background(), conf7)
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		//nolint:dupl
		conf7 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
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
		conf8 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
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

		boardNames := []resource.Name{mockNamed("board1"), mockNamed("board2")}
		motorNames := []resource.Name{mockNamed("m1")}
		encoderNames := []resource.Name{mockNamed("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"), mockNamed("mock6"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			encoderNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("arm1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("arm2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		//nolint:dupl
		conf7 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
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
		conf9 := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"pins": map[string]interface{}{
							"a": "encoder",
							"b": "encoder-b",
						},
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "armFake",
					API:       mockAPI,
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

		boardNames := []resource.Name{mockNamed("board1"), mockNamed("board2")}
		motorNames := []resource.Name{mockNamed("m1")}
		encoderNames := []resource.Name{mockNamed("e1")}
		mockNames := []resource.Name{
			mockNamed("mock1"), mockNamed("mock2"),
			mockNamed("mock3"), mockNamed("mock4"), mockNamed("mock5"),
			mockNamed("mock6"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err := robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m3"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("m4"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base1"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("base2"))
		test.That(t, err, test.ShouldNotBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		_, err = robot.ResourceByName(mockNamed("armFake"))
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
				mockNamed("armFake"),
				mockNamed("mock1"),
				mockNamed("mock4"),
				mockNamed("mock5"),
				mockNamed("mock6"),
			},
		))

		// This configuration will put `mock6` into a good state after two calls to "reconfigure".
		conf9good := processConfig(t, &config.Config{
			Components: []resource.Config{
				{
					Name:  "board2",
					API:   mockAPI,
					Model: model1,
				},
				{
					Name:  "board1",
					API:   mockAPI,
					Model: model1,
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
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
						"encoder":  "e1",
						"pins": map[string]interface{}{
							"pwm": "5",
							"dir": "2",
						},
						"value":              4000,
						"max_rpm":            60,
						"ticks_per_rotation": 1,
					},
					DependsOn: []string{"board1", "e1"},
				},
				{
					Name:  "e1",
					API:   mockAPI,
					Model: mockWithDepModel,
					Attributes: rutils.AttributeMap{
						"mock_dep": "board1",
					},
					DependsOn: []string{"board1"},
				},
				{
					Name:      "armFake",
					API:       mockAPI,
					Model:     model1,
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

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			boardNames,
			resource.DefaultServices(),
			motorNames,
			mockNames,
			encoderNames,
		))
		rdktestutils.VerifySameElements(t, robot.ProcessManager().ProcessIDs(), []string{"1", "2"})

		_, err = robot.ResourceByName(mockNamed("board1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("m1"))
		test.That(t, err, test.ShouldBeNil)

		_, err = robot.ResourceByName(mockNamed("board2"))
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
		_, err = robot.ResourceByName(mockNamed("armFake"))
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
			armFake, err := robot.ResourceByName(mockNamed("armFake"))
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
				mockNamed("armFake"),
				mockNamed("mock6"),
			},
		))
	})
	t.Run("complex diff", func(t *testing.T) {
		resetComponentFailureState()
		logger := logging.NewTestLogger(t)
		conf1 := processConfig(t, &config.Config{
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
					API:   mockAPI,
					Model: model1,
				},
			},
		})
		conf2 := processConfig(t, &config.Config{
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

		armNames := []resource.Name{mockNamed("mock7")}
		mockNames := []resource.Name{
			mockNamed("mock3"), mockNamed("mock4"),
			mockNamed("mock6"), mockNamed("mock5"),
		}

		robot.Reconfigure(context.Background(), conf1)
		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			armNames,
			resource.DefaultServices(),
			mockNames,
		))
		_, err := robot.ResourceByName(mockNamed("mock1"))
		test.That(t, err, test.ShouldNotBeNil)
		_, err = robot.ResourceByName(mockNamed("mock7"))
		test.That(t, err, test.ShouldBeNil)

		robot.Reconfigure(context.Background(), conf2)
		mockNames = []resource.Name{
			mockNamed("mock1"),
			mockNamed("mock3"), mockNamed("mock2"), mockNamed("mock5"),
		}
		test.That(t, robot.RemoteNames(), test.ShouldBeEmpty)

		rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(), rdktestutils.ConcatResourceNames(
			mockNames,
			resource.DefaultServices(),
		))

		_, err = robot.ResourceByName(mockNamed("arm1"))
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

func TestDefaultServiceReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	motionName1 := "motion1"
	motionName2 := "motion2"
	cfg1 := &config.Config{
		Services: []resource.Config{
			{
				Name:  motionName1,
				API:   motion.API,
				Model: resource.DefaultServiceModel,
			},
			{
				Name:  motionName2,
				API:   motion.API,
				Model: resource.DefaultServiceModel,
			},
		},
	}
	robot := setupLocalRobot(t, context.Background(), cfg1, logger)

	rdktestutils.VerifySameResourceNames(t, robot.ResourceNames(),
		[]resource.Name{
			motion.Named(motionName1),
			motion.Named(motionName2),
			motion.Named(resource.DefaultServiceName),
		},
	)

	cfg2 := &config.Config{}
	robot.Reconfigure(context.Background(), cfg2)
	rdktestutils.VerifySameResourceNames(
		t,
		robot.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
		},
	)
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

	type testHarness struct {
		ctx context.Context

		wg             sync.WaitGroup
		cfg            *config.Config
		constructCount int
	}

	setupTest := func(t *testing.T) *testHarness {
		var th testHarness

		ctx, cancelFunc := context.WithCancel(context.Background())
		t.Cleanup(func() { cancelFunc() })
		th.ctx = ctx

		model1 := registerMockComponent(
			t,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{
				Constructor: func(
					ctx context.Context,
					deps resource.Dependencies,
					conf resource.Config,
					logger logging.Logger,
				) (resource.Resource, error) {
					th.constructCount++
					th.wg.Add(1)
					defer th.wg.Done()
					cancelFunc()
					<-ctx.Done()
					return &mockFake{Named: conf.ResourceName().AsNamed()}, nil
				},
			})

		th.cfg = &config.Config{
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
		return &th
	}

	t.Run("new", func(t *testing.T) {
		th := setupTest(t)
		setupLocalRobot(t, th.ctx, th.cfg, logger)

		th.wg.Wait()
		test.That(t, th.constructCount, test.ShouldEqual, 1)
	})
	t.Run("reconfigure", func(t *testing.T) {
		th := setupTest(t)
		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)
		test.That(t, th.constructCount, test.ShouldEqual, 0)

		r.Reconfigure(th.ctx, th.cfg)
		th.wg.Wait()
		test.That(t, th.constructCount, test.ShouldEqual, 1)
	})
}

// tests that on context done, the newly constructed resource gets closed.
func TestResourceConstructCtxDone(t *testing.T) {
	logger := logging.NewTestLogger(t)

	type testHarness struct {
		ctx context.Context

		cfg            *config.Config
		constructCount int
		mf             *mockFake
	}

	setupTest := func(t *testing.T, shouldCancel bool) *testHarness {
		var th testHarness

		ctx, cancelFunc := context.WithCancel(context.Background())
		t.Cleanup(func() { cancelFunc() })
		th.ctx = ctx

		model1 := registerMockComponent(
			t,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{
				Constructor: func(
					ctx context.Context,
					deps resource.Dependencies,
					conf resource.Config,
					logger logging.Logger,
				) (resource.Resource, error) {
					th.constructCount++
					if shouldCancel {
						cancelFunc()
						<-ctx.Done()
					}
					return th.mf, nil
				},
			})
		mock1Cfg := resource.Config{
			Name:  "one",
			Model: model1,
			API:   mockAPI,
		}

		th.mf = &mockFake{Named: mock1Cfg.ResourceName().AsNamed()}
		th.cfg = &config.Config{Components: []resource.Config{mock1Cfg}}
		return &th
	}

	t.Run("new and add normally", func(t *testing.T) {
		th := setupTest(t, false)
		setupLocalRobot(t, th.ctx, th.cfg, logger)

		test.That(t, th.constructCount, test.ShouldEqual, 1)
		test.That(t, th.mf.closeCount, test.ShouldEqual, 0)
	})

	t.Run("reconfiguring and add resource normally", func(t *testing.T) {
		th := setupTest(t, false)

		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)

		test.That(t, th.constructCount, test.ShouldEqual, 0)
		test.That(t, th.mf.closeCount, test.ShouldEqual, 0)

		// test adding the resource during Reconfigure because a new local robot uses the context passed in
		// for many other routines, so cancelling that creates a lot of side effects.
		r.Reconfigure(th.ctx, th.cfg)

		// wait for reconfigureWorkers here since we cancelled the context and
		// the robot doesn't wait for reconfigureWorkers to complete to return from Reconfigure,
		// but resource close is done by a reconfigureWorker routine.
		lRobot, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		lRobot.reconfigureWorkers.Wait()

		test.That(t, th.constructCount, test.ShouldEqual, 1)
		test.That(t, th.mf.closeCount, test.ShouldEqual, 0)
	})

	t.Run("reconfiguring and cancel context during resource add", func(t *testing.T) {
		th := setupTest(t, true)

		r := setupLocalRobot(t, context.Background(), &config.Config{}, logger)

		test.That(t, th.constructCount, test.ShouldEqual, 0)
		test.That(t, th.mf.closeCount, test.ShouldEqual, 0)

		// test adding the resource during Reconfigure because a new local robot uses the context passed in
		// for many other routines, so cancelling that creates a lot of side effects.
		r.Reconfigure(th.ctx, th.cfg)

		// have to wait for reconfigureWorkers here since we cancelled the context and
		// the robot doesn't wait for reconfigureWorkers to complete.
		lRobot, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		lRobot.reconfigureWorkers.Wait()

		test.That(t, th.constructCount, test.ShouldEqual, 1)
		test.That(t, th.mf.closeCount, test.ShouldEqual, 1)
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
	Value            int

	// this field can be set by dependent resources.
	childValues map[string]int
	mu          sync.Mutex
}

type mockFakeConfig struct {
	InferredDep           []string `json:"inferred_dep"`
	ShouldFail            bool     `json:"should_fail"`
	ShouldFailReconfigure int      `json:"should_fail_reconfigure"`
	Blah                  int      `json:"blah"`
	Value                 int      `json:"value"`
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

func (m *mockFake) SetChildValue(slot string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.childValues[slot] = value
}

func (m *mockFake) GetChildValue(slot string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.childValues[slot]
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

// mockWithDep is a mock dependency that directly updates a parent mockFake. This is
// meant to approximate the relationship of a fake motor to a fake board, since the
// former can directly update the pin values of the latter.
type mockWithDep struct {
	resource.Named
	parent      *mockFake
	Slot        string
	Value       int
	reconfCount int
	closeCount  int
}

type mockWithDepConfig struct {
	MockDep string `json:"mock_dep"`
	Slot    string `json:"slot"`
	Value   int    `json:"value"`
}

func (m *mockWithDep) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	m.reconfCount++
	convAttrs := conf.ConvertedAttributes.(*mockWithDepConfig)
	mockDepName := convAttrs.MockDep
	mockDep, ok := deps[mockNamed(mockDepName)]
	if !ok {
		return errors.New("missing dependency")
	}
	parent := mockDep.(*mockFake)
	m.parent.SetChildValue(m.Slot, 0)
	m.parent = parent
	m.Slot = convAttrs.Slot
	m.Value = convAttrs.Value
	m.parent.SetChildValue(m.Slot, m.Value)
	return nil
}

func (m *mockWithDep) Close(ctx context.Context) error {
	m.closeCount++
	m.parent.SetChildValue(m.Slot, 0)
	return nil
}

func (m *mockWithDepConfig) Validate(path string) ([]string, error) {
	depOut := []string{}
	depOut = append(depOut, m.MockDep)
	return depOut, nil
}
