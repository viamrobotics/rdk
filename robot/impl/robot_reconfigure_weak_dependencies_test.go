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
	// this resource will depend on every `component` resource.
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
	// matcher, this `base1` component will be parsed as a weak dependency of weak1.
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

	weakRes, err := robot.ResourceByName(weak1Name)
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
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

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	// With two other components, weak1 now has two (weak) dependencies.
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
				// We need the following `robot.Reconfigure` to call `Reconfigure` on this weak1
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
	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
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

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
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
	// this resource will depend on every `component` resource.
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
	// (that also has an explicit dependency on one of the components).
	// The following scenario is expected:
	// 1) base1 and base2 configure first in parallel, alongside any default resources (motion).
	// 2) Prior to calling the constructor for the resources and subsequently `SwapResource` (which bumps the
	//    logical clock), `completeConfig` checks to see if there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base1, base2, motion)
	//      depends on at least one resource with weak dependencies (weak dependents)
	//    - The logical clock (0) is higher than the `lastWeakDependentsRound` (0) value
	//
	//    Both conditions are false. There will be no call to `updateWeakDependents`. because the logical
	//    clock has not changed and there is nothing that depends on weak dependents, e.g., weak1 in
	//    the current level.
	//
	//    base1, base2, and motion is configured and the logical clock is bumped to 3.
	// 3) weak1 will be processed in a separate reconfiguration level. Prior to that, `completeConfig` checks if
	//    there is a need to call `updateWeakDependents`. For the second level, the logical clock (3) has changed
	//    since the `lastWeakDependentsRound` (0) value but resources (weak1) in the reconfiguration level do not
	//    depend on resources with weak dependencies. Therefore, there will be no call to `updateWeakDependents`.
	// 4) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will now be reconfigured (to increase weak1.reconfigCount to 1) and `lastWeakDependentsRound`
	//    will be updated to 4.

	t.Log("Robot startup")
	base1Name := base.Named("base1")
	base2Name := base.Named("base2")
	weakCfg := config.Config{
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
	robot := setupLocalRobot(t, context.Background(), &weakCfg, logger)

	weakRes, err := robot.ResourceByName(weak1Name)
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	// Assert that the weak dependency was tracked.
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 1)

	lRobot := robot.(*localRobot)
	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 4)

	// Introduce a config diff for base1. This test serves to ensure that updating the configuration of
	// a "explicit" dependency of a resource with weak dependencies functions as expected. The following scenario is expected:
	// 1) base1 needs to reconfigure, and will be processed in the first reconfiguration level, alongside
	//    base2 and motion (neither of which needs to be reconfigured because their configs did not change).
	// 2) Prior to calling `base1.Reconfigure` and subsequently `SwapResource` (which bumps the
	//    logical clock), `completeConfig` checks to see if there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base1)
	//      depends on at least one resource with weak dependencies
	//    - The logical clock (4) is higher than the `lastWeakDependentsRound` (4) value
	//
	//    Both conditions are false. There will be no call to `updateWeakDependents`. because the logical
	//    clock has not changed and there is nothing that depends on weak dependents, e.g., weak1 in
	//    the current level.
	//
	//    base1 is reconfigured and the logical clock is bumped to 5.
	// 3) weak1 will be processed in a separate reconfiguration level but it will not reconfigure.
	//    For a resource to reconfigure, one of the following conditions should be met:
	//    - Resource config has a diff in the new robot config
	//    - An "explicit" dependent reconfigured and resulted in either of the following conditions:
	//      - an error return value
	//      - a new resource object being created ("newly built"). `base1` is reconfigured "in place".
	//
	//    weak1's config does not have a diff and its "explicit" dependents reconfigured "in place", so
	//    it will not reconfigure. Because weak 1 will not reconfigure, the check for whether to call
	//    `updateWeakDependents` will also fail.
	// 4) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will be reconfigured (to increase weak1.reconfigCount to 2) and `lastWeakDependentsRound`
	//    will be updated to 5.
	t.Log("Reconfigure base1")
	weakCfg.Components[1].Attributes = rutils.AttributeMap{"version": 1}
	robot.Reconfigure(context.Background(), &weakCfg)

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 5)

	// Introduce a config diff for base2. The test serves to ensure that updating the configuration of
	// a weak dependency functions as expected. The following scenario is expected:
	// 1) base2 needs to reconfigure, and will be processed in the first reconfiguration level, alongside
	//    base1 and motion (neither of which needs to be reconfigured because their configs did not change).
	// 2) Prior to calling `base2.Reconfigure` and subsequently `SwapResource` (which bumps the
	//    logical clock), `completeConfig` checks to see if there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base2)
	//      depends on at least one resource with weak dependencies
	//    - The logical clock (5) is higher than the `lastWeakDependentsRound` (5) value
	//
	//    Both conditions are false. There will be no call to `updateWeakDependents`. because the logical
	//    clock has not changed and there is nothing that depends on weak dependents, e.g., weak1 in
	//    the current level.
	//
	//    base2 is reconfigured and the logical clock is bumped to 6.
	// 3) weak1 will be processed in a separate reconfiguration level but it will not reconfigure because
	//    weak1's config does not have a diff and its "explicit" dependents did not reconfigure. Because weak1
	//    will not reconfigure, the check for whether to call `updateWeakDependents` will also fail.
	// 4) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will be reconfigured (to increase weak1.reconfigCount to 3) and `lastWeakDependentsRound`
	//    will be updated to 6.
	t.Log("Reconfigure base2")
	weakCfg.Components[2].Attributes = rutils.AttributeMap{"version": 1}
	robot.Reconfigure(context.Background(), &weakCfg)

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)

	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 6)

	// check that calling getDependencies for either weak1 or base1 does not have side effects
	// such as calling `updateWeakDependents` and changing weak1.reconfigCount
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

	// Register a `Resource` that generates weak dependencies. Specifically, instances of
	// this resource will depend on every `component` resource.
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
	// and one resource with weak dependencies.
	// The following scenario is expected:
	// 1) base2 and weak1 configure first in parallel, alongside any default resources (motion).
	// 2) Prior to calling the constructor for the resources and subsequently `SwapResource` (which bumps the
	//    logical clock), `completeConfig` checks to see if there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base2, weak1, motion)
	//      depends on at least one resource with weak dependencies (weak dependents)
	//    - The logical clock (0) is higher than the `lastWeakDependentsRound` (0) value
	//
	//    Both conditions are false. There will be no call to `updateWeakDependents`. because the logical
	//    clock has not changed and there is nothing that depends on weak dependents, e.g., weak1 in
	//    the current level.
	//
	//    base2, weak1, and motion is configured and the logical clock is bumped to 3.
	// 3) base1 will be processed in a separate reconfiguration level. Prior to that, `completeConfig` checks if
	//    there is a need to call `updateWeakDependents`. For the second level, the logical clock (3) has changed
	//    since the `lastWeakDependentsRound` (0) value and resources (base1) in the reconfiguration level do
	//    depend on resources with weak dependencies (weak1). Therefore, there will be a call to `updateWeakDependents`.
	//    lastWeakDependentsRound will be updated to 3 and weak1.reconfigCount will increase to 1.
	//
	//    After that, base1 will be configured and logical clock is bumped to 4.
	// 4) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will now be reconfigured (to increase weak1.reconfigCount to 2) and `lastWeakDependentsRound`
	//    will be updated to 4.
	t.Log("Robot startup")
	base1Name := base.Named("base1")
	base2Name := base.Named("base2")
	weakCfg := config.Config{
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
	robot := setupLocalRobot(t, context.Background(), &weakCfg, logger)

	weakRes, err := robot.ResourceByName(weak1Name)
	// The resource was found and all dependencies were properly resolved.
	test.That(t, err, test.ShouldBeNil)
	weak1, err := resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	// Assert that the weak dependency was tracked.
	test.That(t, weak1.resources, test.ShouldHaveLength, 2)
	test.That(t, weak1.resources, test.ShouldContainKey, base1Name)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 2)

	lRobot := robot.(*localRobot)
	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 4)

	// Introduce a config diff for base1. This test serves to ensure that updating the configuration of
	// a resource with a dependency on a resource with weak dependencies functions as expected.
	// The following scenario is expected:
	// 1) base2, weak1, and motion will be processed in the first reconfiguration level and none of them
	//    will reconfigure as their configs did not change.
	//
	//    Because none of the resources will reconfigure, the check for whether to call `updateWeakDependents`
	//    will fail. The logical clock will not advance.
	// 2) base1 needs to reconfigure as its config has a diff. Prior to calling `base1.Reconfigure` and
	//    subsequently `SwapResource` (which bumps the logical clock), `completeConfig` checks to see if
	//    there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base1)
	//      depends on at least one resource with weak dependencies
	//    - The logical clock (4) is higher than the `lastWeakDependentsRound` (4) value
	//
	//    The first condition is true while the second is false. There will be no call to `updateWeakDependents`,
	//    because the logical clock has not changed.
	//
	//    base1 is reconfigured and the logical clock is bumped to 5.
	// 3) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will be reconfigured (to increase weak1.reconfigCount to 3) and `lastWeakDependentsRound`
	//    will be updated to 5.

	t.Log("Reconfigure base1")
	weakCfg.Components[1].Attributes = rutils.AttributeMap{"version": 1}
	robot.Reconfigure(context.Background(), &weakCfg)

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 3)

	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 5)

	// Introduce a config diff for base2. The test serves to ensure that updating the configuration of
	// a weak dependency functions as expected. The following scenario is expected:
	// 1) base2 needs to reconfigure, and will be processed in the first reconfiguration level, alongside
	//    weak1 and motion (neither of which needs to be reconfigured because their configs did not change).
	// 2) Prior to calling `base2.Reconfigure` and subsequently `SwapResource` (which bumps the
	//    logical clock), `completeConfig` checks to see if there is a need to call `updateWeakDependents`.
	//    Both conditions below have to be met for `updateWeakDependents` to be called:
	//    - At least one resource that needs to reconfigure in this level (base2)
	//      depends on at least one resource with weak dependencies
	//    - The logical clock (5) is higher than the `lastWeakDependentsRound` (5) value
	//
	//    Both conditions are false. There will be no call to `updateWeakDependents`. because the logical
	//    clock has not changed and there is nothing that depends on weak dependents, e.g., weak1 in
	//    the current level.
	//
	//    base2 is reconfigured and the logical clock is bumped to 6.
	// 3) base1 will be processed in a separate reconfiguration level but it will not reconfigure because
	//    base1's config does not have a diff and its "explicit" dependents did not reconfigure. Because base1
	//    will not reconfigure, the check for whether to call `updateWeakDependents` will also fail.
	// 4) After returning from the "levels" part of reconfiguration, `updateWeakDependents` will be called.
	//    weak1 will be reconfigured (to increase weak1.reconfigCount to 4) and `lastWeakDependentsRound`
	//    will be updated to 6.
	t.Log("Reconfigure base2")
	weakCfg.Components[2].Attributes = rutils.AttributeMap{"version": 1}
	robot.Reconfigure(context.Background(), &weakCfg)

	weakRes, err = robot.ResourceByName(weak1Name)
	test.That(t, err, test.ShouldBeNil)
	weak1, err = resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)

	test.That(t, lRobot.lastWeakDependentsRound.Load(), test.ShouldEqual, 6)

	// check that calling getDependencies for either weak1 or base1 does not have side effects
	// such as calling `updateWeakDependents` and changing weak1.reconfigCount
	node, ok := lRobot.manager.resources.Node(weak1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(weak1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)

	node, ok = lRobot.manager.resources.Node(base1Name)
	test.That(t, ok, test.ShouldBeTrue)
	lRobot.getDependencies(base1Name, node)
	test.That(t, weak1.reconfigCount, test.ShouldEqual, 4)
}
