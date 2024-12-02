package robotimpl

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"
	"go.viam.com/utils"

	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm/fake"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/base"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

type someTypeWithWeakAndStrongDeps struct {
	resource.Named
	resource.TriviallyCloseable
	resources     resource.Dependencies
	reconfigCount int
}

func (s *someTypeWithWeakAndStrongDeps) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	s.resources = deps
	s.reconfigCount++
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
	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weak")
	weakModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
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
	defer resource.Deregister(weakAPI, weakModel)

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
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 1)

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
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

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
	// reconfigCount will not increment as the error happens before Reconfigure is called on the resource.
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

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
	// reconfigCount will reset as the resource was destroyed after the previous configuration failure.
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 1)

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
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)
}

func TestWeakDependentsExplicitDependency(t *testing.T) {
	// This test tests the behavior of weak dependents that have an explicit dependency.
	// The robot has 3 components:
	//   * base1 and base2: two fake base components.
	//   * weak1: a weak component that depends on all components and also has an explicit dependency on base1.
	logger := logging.NewTestLogger(t)

	// Register a `Resource` that generates weak dependencies. Specifically instance of
	// this resource will depend on every `component` resource. See the definition of
	// `internal.ComponentDependencyWildcardMatcher`.
	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weak")
	weakModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
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
	defer resource.Deregister(weakAPI, weakModel)

	// Start robot with two components and one resource with weak dependencies
	// (that also has an implicit dependency on one of the components).
	// The following scenario is expected:
	// 1) The two bases will configure first in parallel (which will each bump the logical clock).
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock on also whether there are dependencies on weak dependents.
	//    The check will fail.
	// 3) weak1 will configure.
	// 4) At the end of reconfiguration, weak1 will reconfigure once.
	t.Log("Robot startup")
	base1Name := base.Named("base1")
	base2Name := base.Named("base2")
	weakCfg1 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				DependsOn:           []string{base1Name.String()},
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
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
	test.That(t, weakCfg1.Ensure(false, logger), test.ShouldBeNil)
	robot := setupLocalRobot(t, context.Background(), &weakCfg1, logger)

	res, err := robot.ResourceByName(weak1Name)
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	// Assert that the weak dependency was tracked.
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 1)

	// Reconfigure base1
	// The following scenario is expected:
	// 1) The base1 will reconfigure (which will bump the logical clock).
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock on also whether there are dependencies on weak dependents.
	//    The check will fail.
	// 3) weak1 will not reconfigure, as base1 was not newly built or error during reconfiguration.
	// 4) At the end of reconfiguration, weak1 will reconfigure once.
	t.Log("Reconfigure base1")
	weakCfg2 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				DependsOn:           []string{base1Name.String()},
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
			},
			{
				Name:  base1Name.Name,
				API:   base.API,
				Model: fake.Model,
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this `base1`
				// component. We change the `Attributes` field from the previous (nil) value to
				// accomplish that.
				Attributes: rutils.AttributeMap{"version": 1},
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg2.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg2)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

	// Reconfigure base2
	// The following scenario is expected:
	// 1) The base2 will reconfigure (which will bump the logical clock).
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock and also whether there are dependencies on weak dependents.
	//    The check will fail.
	// 3) weak1 will not reconfigure, as base1 was not newly built or error during reconfiguration.
	// 4) At the end of reconfiguration, weak1 will reconfigure once.
	t.Log("Reconfigure base2")
	weakCfg3 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				DependsOn:           []string{base1Name.String()},
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
			},
			{
				Name:       base1Name.Name,
				API:        base.API,
				Model:      fake.Model,
				Attributes: rutils.AttributeMap{"version": 1},
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this `base2`
				// component. We change the `Attributes` field from the previous (nil) value to
				// accomplish that.
				Attributes: rutils.AttributeMap{"version": 1},
			},
		},
	}
	test.That(t, weakCfg3.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg3)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)

	// check that getting dependencies for either resource does not increase reconfigCount
	lRobot := robot.(*localRobot)
	node, ok := lRobot.manager.resources.Node(weak1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(weak1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)

	node, ok = lRobot.manager.resources.Node(base1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(base1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)
}

func TestWeakDependentsDependedOn(t *testing.T) {
	// This test tests the behavior of weak dependents that have an explicit dependency.
	// The robot has 3 components:
	//   * base1: a fake base component that has an explicit dependency on weak1.
	//   * base2: a fake base component.
	//   * weak1: a weak component that depends on all components.
	logger := logging.NewTestLogger(t)

	// Register a `Resource` that generates weak dependencies. Specifically instance of
	// this resource will depend on every `component` resource. See the definition of
	// `internal.ComponentDependencyWildcardMatcher`.
	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weak")
	weakModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
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
	defer resource.Deregister(weakAPI, weakModel)

	// Start robot with two components (one of which has an explicit dependency on weak dependents)
	// and one resource with weak dependencies
	// The following scenario is expected:
	// 1) base2 and weak1 will configure first in parallel (which will each bump the logical clock).
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock and also whether there are dependencies on weak dependents.
	//    The check will succeed and weak1 will configure for the first time.
	// 3) The weak component will reconfigure once as part of the reconfiguration cycle.
	// 4) At the end of reconfiguration, weak1 will reconfigure once more.
	t.Log("Robot startup")
	base1Name := base.Named("base1")
	base2Name := base.Named("base2")
	weakCfg1 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
			},
			{
				Name:      base1Name.Name,
				API:       base.API,
				Model:     fake.Model,
				DependsOn: []string{weak1Name.String()},
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg1.Ensure(false, logger), test.ShouldBeNil)
	robot := setupLocalRobot(t, context.Background(), &weakCfg1, logger)

	res, err := robot.ResourceByName(weak1Name)
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	// Assert that the weak dependency was tracked.
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

	// Reconfigure base1
	// The following scenario is expected:
	// 1) base2 and weak1 will attempt reconfigure first in parallel (but no changes were made so neither will
	//    be reconfigured). The logical clock will not be bumped.
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock on also whether there are dependencies on weak dependents.
	//    The check will fail because the logical clock has not changed since the last round of weak dependent
	//    updates.
	// 3) base1 will reconfigure.
	// 4) At the end of reconfiguration, weak1 will reconfigure once.
	t.Log("Reconfigure basè")
	weakCfg2 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
			},
			{
				Name:      base1Name.Name,
				API:       base.API,
				Model:     fake.Model,
				DependsOn: []string{weak1Name.String()},
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this `base1`
				// component. We change the `Attributes` field from the previous (nil) value to
				// accomplish that.
				Attributes: rutils.AttributeMap{"version": 1},
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
			},
		},
	}
	test.That(t, weakCfg2.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg2)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)

	// Reconfigure base2
	// The following scenario is expected:
	// 1) base2 and weak1 will attempt reconfiguration first in parallel (base2 will be reconfigured).
	//    The logical clock will be bumped.
	// 2) The completeConfig loop will check to see if there is a need to updateWeakDependents by
	//    checking the logical clock on also whether there are dependencies on weak dependents.
	//    The check will fail because base1 does not need to be reconfigured.
	// 3) base1 will not reconfigure, as weak1 was not newly built or error during reconfiguration.
	// 4) At the end of reconfiguration, weak1 will reconfigure once more.
	t.Log("Reconfigure base2")
	weakCfg3 := config.Config{
		Components: []resource.Config{
			{
				Name:                weak1Name.Name,
				API:                 weakAPI,
				Model:               weakModel,
				ConvertedAttributes: &someTypeWithWeakAndStrongDepsConfig{},
			},
			{
				Name:       base1Name.Name,
				API:        base.API,
				Model:      fake.Model,
				DependsOn:  []string{weak1Name.String()},
				Attributes: rutils.AttributeMap{"version": 1},
			},
			{
				Name:  base2Name.Name,
				API:   base.API,
				Model: fake.Model,
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this `base2`
				// component. We change the `Attributes` field from the previous (nil) value to
				// accomplish that.
				Attributes: rutils.AttributeMap{"version": 1},
			},
		},
	}
	test.That(t, weakCfg3.Ensure(false, logger), test.ShouldBeNil)
	robot.Reconfigure(context.Background(), &weakCfg3)

	res, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)

	// check that getting dependencies for either resource does not increase reconfigCount
	lRobot := robot.(*localRobot)
	node, ok := lRobot.manager.resources.Node(weak1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(weak1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)

	node, ok = lRobot.manager.resources.Node(base1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(base1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)
}
