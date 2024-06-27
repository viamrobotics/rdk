package robotimpl

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm/fake"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/audioinput"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/base"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/camera"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/generic"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/gripper"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/services/sensors"
	rdktestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

// this serves as a test for updateWeakDependents as the sensors service defines a weak
// dependency.
func TestSensorsServiceReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	// emptyCfg, err := config.Read(context.Background(), "data/diff_config_empty.json", logger)
	emptyCfg := &config.Config{}
	// cfg, err := config.Read(context.Background(), "data/fake.json", logger)
	// test.That(t, err, test.ShouldBeNil)
	cfg := processConfig(t, &config.Config{
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
			},
			{
				Name:  "pieceArm",
				API:   mockAPI,
				Model: resource.DefaultModelFamily.WithModel("fake"),
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
