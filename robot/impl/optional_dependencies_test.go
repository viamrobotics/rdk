package robotimpl

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils"
	gotestutils "go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

// Contains a required and optional motor that the component will necessarily and
// optionally depend upon.
type optionalChildConfig struct {
	RequiredMotor string `json:"required_motor"`
	OptionalMotor string `json:"optional_motor"`
}

// Validate validates the config and returns a required dependency on `required_motor` and
// an optional dependency on `optional_motor`.
func (ocCfg *optionalChildConfig) Validate(path string) ([]string, []string, error) {
	var requiredDeps, optionalDeps []string

	if ocCfg.RequiredMotor == "" {
		return nil, nil,
			fmt.Errorf(`expected "required_motor" attribute for foo %q`, path)
	}
	requiredDeps = append(requiredDeps, ocCfg.RequiredMotor)

	if ocCfg.OptionalMotor != "" {
		optionalDeps = append(optionalDeps, ocCfg.OptionalMotor)
	}

	return requiredDeps, optionalDeps, nil
}

type optionalChild struct {
	resource.Named
	resource.TriviallyCloseable

	logger logging.Logger

	requiredMotor motor.Motor
	optionalMotor motor.Motor
	reconfigCount int
}

func newOptionalChild(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (*optionalChild, error) {
	oc := &optionalChild{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := oc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return oc, nil
}

func (oc *optionalChild) Reconfigure(ctx context.Context, deps resource.Dependencies,
	conf resource.Config,
) error {
	oc.reconfigCount++

	optionalChildConfig, err := resource.NativeConfig[*optionalChildConfig](conf)
	if err != nil {
		return err
	}

	oc.requiredMotor, err = motor.FromProvider(deps, optionalChildConfig.RequiredMotor)
	if err != nil {
		return fmt.Errorf("could not get required motor %s from dependencies",
			optionalChildConfig.RequiredMotor)
	}

	oc.optionalMotor, err = motor.FromProvider(deps, optionalChildConfig.OptionalMotor)
	if err != nil {
		oc.logger.Infof("could not get optional motor %s from dependencies; continuing",
			optionalChildConfig.OptionalMotor)
	}

	return nil
}

func TestOptionalDependencies(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	// Register the optional child component defined above and defer its deregistration.
	optionalChildModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	ocName := generic.Named("oc")
	resource.Register(
		generic.API,
		optionalChildModel,
		resource.Registration[*optionalChild, *optionalChildConfig]{
			Constructor: newOptionalChild,
		})
	defer resource.Deregister(generic.API, optionalChildModel)

	// Reconfigure the robot to have an optional child component, its required motor, and no
	// optional motor.
	cfg := config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	// Ensure here and for all configs below before `Reconfigure`ing to make sure optional
	// dependencies are calculated (`ImplicitOptionalDependsOn` is filled in).
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the optional child component built successfully (optional dependency on
		// non-existent 'm1' did not cause a failure to build).
		ocRes, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the optional child reconfigured and logged its inability to get 'm1'
		// from dependencies exactly once, from construction. updateWeakAndOptionalDependents
		// skips an unnecessary reconfigure here because the resolved set of optional
		// dependencies has not changed since construction (m1 still does not exist).
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 1)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that, on the component itself, `requiredMotor` is now set, but `optionalMotor`
		// is not.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldBeNil)
	}

	// Reconfigure the robot to have the optionally-depended-upon motor 'm1'.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the optional child component is still accessible (did not fail to
		// reconfigure).
		ocRes, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the optional child has reconfigured _twice_. One from the previous
		// construction and one from updateWeakAndOptionalDependents picking up that 'm1'
		// became resolvable. Assert that there were no more logs (still 1) about failures
		// to "get optional motor" since the reconfigure succeeded in supplying 'm1'.
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 2)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that, on the component itself, `requiredMotor` and `optionalMotor` are now
		// both set.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
	}

	// Reconfigure the robot to remove 'm1'.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the optional child component is still accessible (did not fail to
		// reconfigure).
		ocRes, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the optional child has reconfigured three times. Two from the previous
		// construction and reconfigure, and one from the most recent reconfigure to pass
		// _only_ 'm' as a dependency (no 'm1'). Assert that there was another log (now 2)
		// about failures to "get optional motor."
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 3)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

		// Assert that, on the component itself, `requiredMotor` is still set but
		// `optionalMotor` is not.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldBeNil)
	}

	// Reconfigure the robot to remove 'm' (required dependency).
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "m1",
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the optional child component is no longer accessible (failed to
		// reconfigure and can no longer be found).
		_, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	}

	// Reconfigure the robot to add 'm' _and_ 'm1' back.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the optional child component built successfully.
		ocRes, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the optional child reconfigured either once or twice depending on
		// build order. If construction already saw 'm1', updateWeakAndOptionalDependents
		// finds the same snapshot and skips a reconfigure (1 total). If construction did
		// not see 'm1', the snapshot differs and one reconfigure happens (2 total).
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldBeIn, []int{1, 2})

		// Assert that there are either 2 (no new) _or_ 3 logs about an inability to "get
		// optional motor." Two carry over from the prior step. A third appears only when
		// construction order was m -> oc -> m1 (oc constructed without m1).
		//
		// Optional dependencies are _not_ represented as edges in the resource graph and have
		// no influence on build order. 2 logs would mean the order was m -> m1 -> oc or m1 ->
		// m -> oc. 3 logs would mean the order was m -> oc -> m1.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldBeIn, []int{2, 3})

		// Assert that, on the component itself, `requiredMotor` and `optionalMotor` are now
		// set.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
	}

	// Reconfigure 'm1' in place by giving it a config diff so that it remains resolvable
	// in oc's optional depenency set but its GraphNode clock advances via SwapResource,
	// so oc's recorded snapshot of m1's clock goes stale. This exercises the clock-value-mismatch
	// branch of weakOptionalDepClocksEqual — the steps above only add or remove a key,
	// never bump the clock of a key that stays present.
	cfg.Components[2].Attributes = rutils.AttributeMap{"version": 1}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)

	ocRes, err := lr.ResourceByName(ocName)
	test.That(t, err, test.ShouldBeNil)
	oc, err := resource.AsType[*optionalChild](ocRes)
	test.That(t, err, test.ShouldBeNil)
	countBeforeInPlace := oc.reconfigCount
	missingLogsBeforeInPlace := logs.FilterMessageSnippet("could not get optional motor").Len()

	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		ocRes, err := lr.ResourceByName(ocName)
		test.That(t, err, test.ShouldBeNil)
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)

		// 'm1's in-place reconfigure bumped its clock, so oc reconfigures exactly once.
		test.That(t, oc.reconfigCount, test.ShouldEqual, countBeforeInPlace+1)
		// 'm1' stayed resolvable, so no new "could not get optional motor" log.
		test.That(t, logs.FilterMessageSnippet("could not get optional motor").Len(),
			test.ShouldEqual, missingLogsBeforeInPlace)
		// Both motors remain set.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
	}
}

func TestModularOptionalDependencies(t *testing.T) {
	// A copy of TestOptionalDependencies with a modular component instead of a resource
	// defined in this file.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")

	// Reconfigure the robot to have a foo component, its required motor, and no optional
	// motor.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	// Ensure here and for all configs below before `Reconfigure`ing to make sure optional
	// dependencies are calculated (`ImplicitOptionalDependsOn` is filled in).
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the foo component built successfully (optional dependency on
		// non-existent 'm1' did not cause a failure to build).
		fooRes, err := lr.ResourceByName(fooName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the foo component logged its inability to get 'm1' from dependencies
		// exactly once, from construction. updateWeakAndOptionalDependents skips an
		// unnecessary reconfigure here because the resolved set of optional dependencies
		// has not changed since construction (m1 still does not exist).
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that 'm' is accessible through the foo component and not moving.
		doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

		// Assert that 'm1' is not accessible through the foo component.
		doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "unset"})
	}

	// Reconfigure the robot to have the optionally-depended-upon motor 'm1'.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the foo component is still accessible (did not fail to reconfigure).
		fooRes, err := lr.ResourceByName(fooName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that there were no more logs (still 1) about failures to "get optional
		// motor."
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that 'm' is still accessible through the foo component and not moving.
		doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

		// Assert that 'm1' is now accessible through the foo component and not moving.
		doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
	}

	// Reconfigure the robot to remove 'm1'.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the foo component is still accessible (did not fail to reconfigure).
		fooRes, err := lr.ResourceByName(fooName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that there was another log (now 2) about a failure to "get optional
		// motor." Removing 'm1' changes the resolved optional dep set, so foo is
		// reconfigured and re-emits the missing-optional-motor log.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

		// Assert that 'm' is still accessible through the foo component and not moving.
		doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

		// Assert that 'm1' is no longer accessible through the foo component.
		doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "unset"})
	}

	// Reconfigure the robot to remove 'm' (required dependency).
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the foo component is no longer accessible (did not fail to reconfigure).
		_, err := lr.ResourceByName(fooName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	}

	// Reconfigure the robot to add 'm' _and_ 'm1' back.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the foo component built successfully.
		fooRes, err := lr.ResourceByName(fooName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that there are either 2 (no new) _or_ 3 logs about an inability to "get
		// optional motor." Two carry over from the prior steps. A third appears only when
		// construction order was m -> f -> m1 (f constructed without m1, then
		// updateWeakAndOptionalDependents reconfigures with m1 — but the initial missing-
		// m1 log was emitted by construction).
		//
		// Optional dependencies are _not_ represented as edges in the resource graph and have
		// no influence on build order. 2 logs would mean the order was m -> m1 -> f or m1 ->
		// m -> f. 3 logs would mean the order was m -> f -> m1.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldBeIn, []int{2, 3})

		// Assert that 'm' is accessible through the foo component and not moving.
		doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

		// Assert that 'm1' is accessible through the foo component and not moving.
		doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
	}
}

func TestOptionalDependencyOnBuiltin(t *testing.T) {
	// This test ensures that a component can optionally depend upon a resource named
	// "builtin". This validates that the optional dependency system works correctly when the
	// dependency name is "builtin", which could be confused with internal builtin services
	// but is actually just a regular resource with that name.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	// Register the optional child component defined above and defer its deregistration.
	optionalChildModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	ocName := generic.Named("oc")
	resource.Register(
		generic.API,
		optionalChildModel,
		resource.Registration[*optionalChild, *optionalChildConfig]{
			Constructor: newOptionalChild,
		})
	defer resource.Deregister(generic.API, optionalChildModel)

	// Reconfigure the robot to have an optional child component with a required motor 'm'
	// and an optional dependency on a motor named "builtin" (which already exists).
	cfg := config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m",
					OptionalMotor: "builtin",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "builtin",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the optional child component built successfully.
	ocRes, err := lr.ResourceByName(ocName)
	test.That(t, err, test.ShouldBeNil)

	// Assert that the optional child reconfigured either once or twice depending on
	// build order. If construction already saw "builtin", updateWeakAndOptionalDependents
	// finds the same snapshot and skips a reconfigure (1 total). Otherwise the snapshot
	// differs and a single reconfigure happens (2 total).
	oc, err := resource.AsType[*optionalChild](ocRes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, oc.reconfigCount, test.ShouldBeIn, []int{1, 2})

	// Assert that there is either 0 or 1 log about an inability to "get optional motor."
	//
	// The optional child _might_ get 'builtin' as a dependency as part of its initial
	// construction (if builtin initializes first), in which case no log will be emitted, or
	// it _might_ get 'builtin' as a dependency only during the reconfigure triggered by
	// `updateWeakAndOptionalDependents` detecting that the snapshot changed, in which case
	// one log will be emitted due to the initial construction lacking the 'builtin'
	// dependency.
	//
	// Optional dependencies are _not_ represented as edges in the resource graph and have no
	// influence on build order. 0 logs would mean the order was m -> builtin -> oc. 1 log
	// would mean the order was m -> oc -> builtin (or builtin -> m -> oc).
	msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
	test.That(t, msgNum, test.ShouldBeIn, []int{0, 1})

	// Assert that, on the component itself, both `requiredMotor` and `optionalMotor` are set.
	test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
	test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
}

func TestModularOptionalDependencyOnBuiltin(t *testing.T) {
	// This test ensures that a modular component can optionally depend upon a resource named
	// "builtin". This is a modular version of TestOptionalDependencyOnBuiltin.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")

	// Reconfigure the robot to have a foo component with a required motor 'm' and an
	// optional dependency on a motor named "builtin".
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "builtin",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "builtin",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)

	// Assert that there are either 0 or 1 logs about an inability to "get optional motor."
	// With serial configuration (WithDisableCompleteConfigWorker), the build order is more
	// predictable but still depends on module startup timing. 0 logs means 'builtin' was
	// fully available during construction. 1 log means 'builtin' was available during the
	// updateWeakAndOptionalDependents reconfigure.
	msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
	test.That(t, msgNum, test.ShouldBeIn, []int{0, 1})

	// Assert that 'm' is accessible through the foo component and not moving.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	// Assert that 'builtin' is accessible through the foo component and not moving.
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

func TestModularOptionalDependencyOnRemote(t *testing.T) {
	// Ensures that a modular resource can optionally depend upon a remote resource.
	//
	// In this case, the modular resource will be constructed with the remote resource as a
	// dependency since it will be available at the time of construction. Because the
	// resolved set of optional dependencies is unchanged after construction,
	// updateWeakAndOptionalDependents does not trigger a follow-up reconfigure.
	//
	// Later, if the remote goes offline, the modular resource will NOT be reconfigured and
	// instead will just have an unusable gRPC client for the remote resource until the
	// remote comes back online. This behavior is distinct from optionally depending upon a
	// local resource, where the dependent resource _will_ be reconfigured in the event that
	// the local resource is removed (see above test). The justification for this difference
	// in behavior is that we don't want network blips to cause constant reconfigures of
	// resources optionally depending on remote resources.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Set up a remote robot with a motor we will depend on optionally.
	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	// Hold remote's port so it survives remote's Close below and remote2 can reuse
	// the exact same socket, with no window for another process to claim the port.
	options, lis, addr := robottestutils.CreateBaseOptionsAndListener(t)
	hold := testutils.HoldPort(t, lis)
	options.Network.Listener = hold
	err := remote.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Configure a local robot whose modular foo component has a required dependency on a
	// local motor "m" and an optional dependency on the remote motor "m1".
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr,
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr := setupLocalRobot(t, ctx, &cfg, logger.Sublogger("local"), WithDisableCompleteConfigWorker())

	// Assert that the foo component built successfully and was not subsequently
	// reconfigured (its optional dep set was unchanged after construction). Then assert
	// that its optional dependency on the remote motor is reachable.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, logs.FilterMessageSnippet("Reconfiguring resource for module").Len(), test.ShouldEqual, 0)
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})

	// Take the remote robot offline and wait for local robot to notice.
	localResourceNames := slices.DeleteFunc(lr.ResourceNames(), func(name resource.Name) bool {
		return name.ContainsRemoteNames()
	})
	allResourceNames := lr.ResourceNames()
	test.That(t, remote.Close(ctx), test.ShouldBeNil)

	// With the complete config worker disabled, manually trigger remote updates in a loop.
	gotestutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		lr.(*localRobot).updateRemotesAndRetryResourceConfigure()
		verifyReachableResourceNames(tb, lr, localResourceNames)
	})

	// Assert that the foo component did NOT reconfigure but its optional dependency
	// on the remote motor is now unreachable.
	test.That(t, logs.FilterMessageSnippet("Reconfiguring resource for module").Len(), test.ShouldEqual, 0)
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "unreachable"})

	// Bring up a new remote robot on the very same socket remote used. The port was
	// never released, so there was no chance for it to be claimed in between.
	hold.Rearm(t)
	remote2 := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote2"))
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// With the complete config worker disabled, manually trigger remote updates in a loop.
	gotestutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		lr.(*localRobot).updateRemotesAndRetryResourceConfigure()
		verifyReachableResourceNames(tb, lr, allResourceNames)
	})

	// Assert that the foo component did NOT reconfigure, but its optional dependency on the
	// remote motor is now reachable.
	test.That(t, logs.FilterMessageSnippet("Reconfiguring resource for module").Len(), test.ShouldEqual, 0)
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

func TestModularOptionalDependencyOnRemoteWithPrefix(t *testing.T) {
	// Ensures that a modular resource can optionally depend upon a remote resource on a remote with prefix.
	//
	// In this case, the modular resource is constructed with the remote resource as a
	// dependency since it is available at the time of construction. Because the resolved
	// set of optional dependencies is unchanged after construction,
	// updateWeakAndOptionalDependents does not trigger a follow-up reconfigure.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Set up a remote robot with a motor we will depend on optionally.
	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:                "m1",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err := remote.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Configure a local robot whose modular foo component has a required dependency on a
	// local motor "m" and an optional dependency on the remote motor "m1".
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m",
					"optional_motor": "remote-m1",
				},
			},
			{
				Name:                "m",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "remote",
				Address: addr,
				Prefix:  "remote-",
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr := setupLocalRobot(t, ctx, &cfg, logger.Sublogger("local"), WithDisableCompleteConfigWorker())

	// Assert that the foo component built successfully and was not subsequently
	// reconfigured (its optional dep set was unchanged after construction). Then assert
	// that its optional dependency on the remote motor is reachable.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, logs.FilterMessageSnippet("Reconfiguring resource for module").Len(), test.ShouldEqual, 0)
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

func TestModularOptionalDependencyOnFullyQualifiedName(t *testing.T) {
	// Ensures that optional dependencies specified as fully qualified resource names
	// (e.g. "rdk:component:motor/m_optional") are correctly resolved by the robot.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")

	// Configure the robot with foo optionally depending on "m_optional" expressed as its
	// fully qualified resource name. The robot must resolve this FQN to the same motor
	// that a simple short name would.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "rdk:component:motor/m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully and that the FQN was resolved.
	// The reconfigure count is 0 if construction already saw m_optional, or 1 if foo
	// was built before m_optional and updateWeakAndOptionalDependents had to follow up.
	// Either way, the FQN must resolve to the same motor as a simple short name would.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, logs.FilterMessageSnippet("Reconfiguring resource for module").Len(), test.ShouldBeIn, []int{0, 1})

	// Verify foo works and that the optional dependency is reachable.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

// Contains _another_ MOC that this MOC will optionally depend upon.
type mutualOptionalChildConfig struct {
	OtherMOC string `json:"other_moc"`
}

// Validate validates the config and returns an optional dependency on `other_moc`.
func (mocCfg *mutualOptionalChildConfig) Validate(path string) ([]string, []string, error) {
	if mocCfg.OtherMOC == "" {
		return nil, nil,
			fmt.Errorf(`expected "other_moc" attribute for MOC %q`, path)
	}
	return nil, []string{mocCfg.OtherMOC}, nil
}

type mutualOptionalChild struct {
	resource.Named
	resource.TriviallyCloseable

	logger logging.Logger

	otherMOC      resource.Resource
	reconfigCount int
}

func newMutualOptionalChild(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (*mutualOptionalChild, error) {
	moc := &mutualOptionalChild{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := moc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return moc, nil
}

func (moc *mutualOptionalChild) Reconfigure(ctx context.Context, deps resource.Dependencies,
	conf resource.Config,
) error {
	moc.reconfigCount++

	mutualOptionalChildConfig, err := resource.NativeConfig[*mutualOptionalChildConfig](conf)
	if err != nil {
		return err
	}

	moc.otherMOC, err = generic.FromProvider(deps, mutualOptionalChildConfig.OtherMOC)
	if err != nil {
		moc.logger.Infof("could not get other MOC %s from dependencies; continuing",
			mutualOptionalChildConfig.OtherMOC)
	}

	return nil
}

func TestOptionalDependenciesCycles(t *testing.T) {
	// This test ensures that there can be a "cycle" of non-modular optional dependencies.
	// Note that the usage of non-modular optional dependencies requires that the resource
	// have a `Reconfigure` method (defined above).
	//
	// A resource 'moc' will optionally depend upon 'moc2', and 'moc2' will optionally
	// depend upon 'moc'. We will start with only 'moc' in the config, and assert that 'moc'
	// builds successfully without 'moc2'. We will then add 'moc2' to the config, assert
	// that 'moc' reconfigures successfully, 'moc2' builds successfully, and both resources
	// have handles to each other. We will then remove 'moc' from the config, assert that
	// 'moc2' reconfigures successfully, and that 'moc2' no longer has a handle to 'moc'.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	// Register the mutual optional child component defined above and defer its deregistration.
	mutualOptionalChildModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	mocName := generic.Named("moc")
	mocName2 := generic.Named("moc2")
	resource.Register(
		generic.API,
		mutualOptionalChildModel,
		resource.Registration[*mutualOptionalChild, *mutualOptionalChildConfig]{
			Constructor: newMutualOptionalChild,
		})
	defer resource.Deregister(generic.API, mutualOptionalChildModel)

	// Reconfigure the robot to have a mutual optional child component that is missing its
	// mutual.
	cfg := config.Config{
		Components: []resource.Config{
			{
				Name:  mocName.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				ConvertedAttributes: &mutualOptionalChildConfig{
					OtherMOC: mocName2.Name,
				},
			},
		},
	}
	// Ensure here and for all configs below before `Reconfigure`ing to make sure optional
	// dependencies are calculated (`ImplicitOptionalDependsOn` is filled in).
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the mutual optional child component built successfully (optional
		// dependency on non-existent 'moc2' did not cause a failure to build).
		mocRes, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the mutual optional child reconfigured and logged its inability to get
		// 'moc2' from dependencies exactly once, from construction.
		// updateWeakAndOptionalDependents skips an unnecessary reconfigure because the
		// resolved optional dep set is unchanged.
		moc, err := resource.AsType[*mutualOptionalChild](mocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moc.reconfigCount, test.ShouldEqual, 1)
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that, on the component itself, `otherMOC` remains unset.
		test.That(t, moc.otherMOC, test.ShouldBeNil)
	}

	// Reconfigure the robot to have the other MOC.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  mocName.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				ConvertedAttributes: &mutualOptionalChildConfig{
					OtherMOC: mocName2.Name,
				},
			},
			{
				Name:  mocName2.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				ConvertedAttributes: &mutualOptionalChildConfig{
					OtherMOC: mocName.Name,
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the first 'moc' component is still accessible (did not fail to
		// reconfigure).
		mocRes, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the 'moc' mutual optional child has reconfigured _twice_. Once from
		// the previous construction and once from the reconfigure to pass 'moc2' as a
		// dependency. Assert that there were no more logs (still 1) about failures to "get
		// other MOC."
		moc, err := resource.AsType[*mutualOptionalChild](mocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moc.reconfigCount, test.ShouldEqual, 2)
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that, on the 'moc' component itself, `otherMOC` is now set.
		test.That(t, moc.otherMOC, test.ShouldNotBeNil)

		// Assert that the second 'moc2' component is now accessible (did not fail to
		// construct).
		mocRes2, err := lr.ResourceByName(mocName2)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the second mutual optional child has reconfigured exactly once from
		// construction; its resolved optional dep set was unchanged afterwards.
		moc2, err := resource.AsType[*mutualOptionalChild](mocRes2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moc2.reconfigCount, test.ShouldEqual, 1)

		// Assert that, on the 'moc2' component itself, `otherMOC` is now set.
		test.That(t, moc2.otherMOC, test.ShouldNotBeNil)
	}

	// Reconfigure the robot to remove the original 'moc'.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  mocName2.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				ConvertedAttributes: &mutualOptionalChildConfig{
					OtherMOC: mocName.Name,
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the original optional child component 'moc' is no longer accessible.
		_, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

		// Assert that the second optional child component 'moc2' is still accessible.
		mocRes2, err := lr.ResourceByName(mocName2)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the second optional child 'moc2' has reconfigured twice. Once from
		// its construction (when 'moc' was present) and one from the most recent reconfigure
		// to remove 'moc' as a dependency. Assert that there was another log (now 2) about
		// failures to "get other MOC."
		moc2, err := resource.AsType[*mutualOptionalChild](mocRes2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moc2.reconfigCount, test.ShouldEqual, 2)
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

		// Assert that, on the 'moc2' component itself, `otherMOC` is no longer set.
		test.That(t, moc2.otherMOC, test.ShouldBeNil)
	}
}

func TestModularOptionalDependenciesCycles(t *testing.T) {
	// This test is a copy of TestOptionalDependenciesCycles, but it also ensures that
	// modular resources can optionally depend upon each other _and_ lack a `Reconfigure`
	// method (leverage `resource.AlwaysRebuild`).

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	mutualOptionalChildModel := resource.NewModel("acme", "demo", "moc")
	mocName := generic.Named("moc")
	mocName2 := generic.Named("moc2")

	// Reconfigure the robot to have a mutual optional child component that is missing its
	// mutual.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  mocName.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				Attributes: rutils.AttributeMap{
					"other_moc": mocName2.Name,
				},
			},
		},
	}
	// Ensure here and for all configs below before `Reconfigure`ing to make sure optional
	// dependencies are calculated (`ImplicitOptionalDependsOn` is filled in).
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the mutual optional child component built successfully (optional
		// dependency on non-existent 'moc2' did not cause a failure to build).
		mocRes, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the mutual optional logged its inability to get 'moc2' from
		// dependencies exactly once, from construction. updateWeakAndOptionalDependents
		// skips a follow-up rebuild because the resolved optional dep set is unchanged.
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		// Assert that, on the component itself, `otherMOC` remains unset.
		doCommandResp, err := mocRes.DoCommand(ctx, map[string]any{"command": "other_moc_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"other_moc_state": "unset"})
	}

	// Reconfigure the robot to have the other MOC.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  mocName.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				Attributes: rutils.AttributeMap{
					"other_moc": mocName2.Name,
				},
			},
			{
				Name:  mocName2.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				Attributes: rutils.AttributeMap{
					"other_moc": mocName.Name,
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// In a mutual-optional cycle the cascade deliberately stops before re-adding the
		// resource on the visited stack (see cascadeRebuildDependentsOf), so exactly one
		// side ends up with a stale captured pointer. Which side is stale depends on the
		// order updateWeakAndOptionalDependents iterates resources, which is non-deterministic.

		mocRes, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldBeNil)
		mocRes2, err := lr.ResourceByName(mocName2)
		test.That(t, err, test.ShouldBeNil)

		// Assert that there were no more logs (still 1) about failures to "get other MOC."
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 1)

		currentID := func(res resource.Resource) float64 {
			resp, err := res.DoCommand(ctx, map[string]any{"command": "instance_id"})
			test.That(t, err, test.ShouldBeNil)
			return resp["instance_id"].(float64)
		}
		seenOtherID := func(res resource.Resource) float64 {
			resp, err := res.DoCommand(ctx, map[string]any{"command": "other_moc_state"})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, resp["other_moc_state"], test.ShouldEqual, "usable")
			return resp["other_instance_id"].(float64)
		}

		mocCurrent := currentID(mocRes)
		moc2Current := currentID(mocRes2)
		mocSeesMoc2 := seenOtherID(mocRes)
		moc2SeesMoc := seenOtherID(mocRes2)

		// One of moc / moc2 has fresh pointer to the other and the other has stale
		// pointer to the first.
		mocFresh := mocSeesMoc2 == moc2Current
		moc2Fresh := moc2SeesMoc == mocCurrent
		test.That(t, mocFresh != moc2Fresh, test.ShouldBeTrue)
	}

	// Reconfigure the robot to remove the original 'moc'.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  mocName2.Name,
				API:   generic.API,
				Model: mutualOptionalChildModel,
				Attributes: rutils.AttributeMap{
					"other_moc": mocName.Name,
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	{ // Assertions
		// Assert that the original optional child component 'moc' is no longer accessible.
		_, err := lr.ResourceByName(mocName)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

		// Assert that the second optional child component 'moc2' is still accessible (did not
		// fail to reconstruct).
		mocRes2, err := lr.ResourceByName(mocName2)
		test.That(t, err, test.ShouldBeNil)

		// Assert that there was another log (now 2) about failures to "get other MOC."
		msgNum := logs.FilterMessageSnippet("could not get other MOC").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

		// Assert that, on the 'moc2' component itself, `otherMOC` is no longer set.
		doCommandResp, err := mocRes2.DoCommand(ctx, map[string]any{"command": "other_moc_state"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"other_moc_state": "unset"})
	}
}

func TestOptionalDependencyRepeatedErrors(t *testing.T) {
	// This test verifies that when an unrelated resource has repeated errors, resources with
	// optional dependencies do not reconfigure multiple times. This exercises the logical
	// clock increment logic: the clock only increments on state transitions (usable→unusable),
	// not on repeated errors while already unusable.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	// Register the optional child component.
	optionalChildModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	ocName := generic.Named("oc")
	resource.Register(
		generic.API,
		optionalChildModel,
		resource.Registration[*optionalChild, *optionalChildConfig]{
			Constructor: newOptionalChild,
		})
	defer resource.Deregister(generic.API, optionalChildModel)

	// Configure the robot with:
	// - An optional child "oc" that has a required motor "m_required" and optional dependency on "m_optional"
	// - An unrelated motor "m_unrelated" that will experience repeated errors
	cfg := config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m_required",
					OptionalMotor: "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_unrelated",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Get the optional child and verify initial state.
	ocRes, err := lr.ResourceByName(ocName)
	test.That(t, err, test.ShouldBeNil)
	oc, err := resource.AsType[*optionalChild](ocRes)
	test.That(t, err, test.ShouldBeNil)

	// The optional child reconfigured 1 or 2 times depending on construction order:
	// 1 if it saw m_optional during construction, 2 if it had to be reconfigured by
	// updateWeakAndOptionalDependents to pick up m_optional.
	initialReconfigCount := oc.reconfigCount
	test.That(t, initialReconfigCount, test.ShouldBeIn, []int{1, 2})

	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Record reconfigCount before inducing the error.
	reconfigCountBeforeError := oc.reconfigCount

	// Clear any existing logs to make assertions cleaner.
	logs.TakeAll()

	// Reconfigure with an invalid model for m_unrelated to induce a build error.
	// This transitions m_unrelated from usable→unusable and increments the clock. The
	// optional child's resolved optional dep set is unchanged (m_unrelated is unrelated
	// and m_optional is still present), so updateWeakAndOptionalDependents skips it.
	nonExistentModel := resource.NewModel("rdk", "builtin", "nonexistent")
	cfg.Components[3] = resource.Config{
		Name:  "m_unrelated",
		API:   motor.API,
		Model: nonExistentModel,
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	clockAfterFirstError := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, clockAfterFirstError,
		test.ShouldEqual, initialClockValue+1)

	// Verify the first error logged a build failure.
	firstErrorLogs := logs.FilterMessageSnippet("resource build error: unknown resource type").Len()
	test.That(t, firstErrorLogs, test.ShouldEqual, 1)

	// Clear logs again to isolate just the retry attempts.
	logs.TakeAll()

	// Call updateRemotesAndRetryResourceConfigure multiple times.
	// Each call will attempt to retry configuring m_unrelated, which will fail again with the
	// invalid model. These repeated errors should NOT increment the clock (self-transitions
	// in Unhealthy state). Because the clock doesn't change, updateWeakAndOptionalDependents
	// should return early without additional reconfigurations of the optional child.
	for i := 0; i < 5; i++ {
		lr.(*localRobot).updateRemotesAndRetryResourceConfigure()
		// The clock should NOT increment - m_unrelated is still unusable.
		test.That(t, lr.(*localRobot).manager.resources.CurrLogicalClockValue(),
			test.ShouldEqual, clockAfterFirstError)
	}

	// Verify the optional child was not reconfigured by any of the clock changes. The
	// failing m_unrelated is unrelated to the optional child, so its optional dep set
	// (m_optional) is unchanged and updateWeakAndOptionalDependents skips it.
	reconfigCountAfterAllUpdates := oc.reconfigCount
	test.That(t, reconfigCountAfterAllUpdates-reconfigCountBeforeError, test.ShouldEqual, 0)

	// Verify that m_unrelated failed to build 5 times (one for each retry call).
	buildErrorLogs := logs.FilterMessageSnippet("resource build error: unknown resource type").Len()
	test.That(t, buildErrorLogs, test.ShouldEqual, 5)

	// Verify that no "could not get optional motor" logs were emitted, confirming the
	// optional child wasn't repeatedly reconfigured due to the unrelated resource's repeated errors.
	msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
	test.That(t, msgNum, test.ShouldEqual, 0)

	// Verify the key property: Clock incremented only once (for the first error),
	// not for each of the 5 retry attempts. Multiple calls to updateRemotesAndRetryResourceConfigure
	// with unchanged clock resulted in no additional reconfigurations.
	test.That(t, lr.(*localRobot).manager.resources.CurrLogicalClockValue(),
		test.ShouldEqual, clockAfterFirstError)
}

func TestModularOptionalDependencyRepeatedErrors(t *testing.T) {
	// This test is the modular version of TestOptionalDependencyRepeatedErrors. It verifies
	// that when an unrelated resource has repeated errors, modular resources with optional
	// dependencies do not reconfigure multiple times.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")

	// Configure the robot with:
	// - A foo component that has required motor "m_required" and optional dependency on "m_optional"
	// - An unrelated motor "m_unrelated" that will experience repeated errors
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_unrelated",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)

	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Clear any existing logs to make assertions cleaner.
	logs.TakeAll()

	// Reconfigure with an invalid model for m_unrelated to induce a build error.
	// This transitions m_unrelated from usable→unusable, increments the clock, and triggers
	// updateWeakAndOptionalDependents which may reconfigure the foo component once.
	nonExistentModel := resource.NewModel("rdk", "builtin", "nonexistent")
	cfg.Components[3] = resource.Config{
		Name:  "m_unrelated",
		API:   motor.API,
		Model: nonExistentModel,
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)
	test.That(t, lr.(*localRobot).manager.resources.CurrLogicalClockValue(),
		test.ShouldEqual, initialClockValue+1)
	clockAfterFirstError := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Verify the first error logged a build failure.
	firstErrorLogs := logs.FilterMessageSnippet("resource build error: unknown resource type").Len()
	test.That(t, firstErrorLogs, test.ShouldEqual, 1)

	// Verify the foo component was NOT reconfigured by the clock change. The failing
	// m_unrelated is unrelated to foo, so foo's optional dep set is unchanged and
	// updateWeakAndOptionalDependents skips it.
	reconfigLogsAfterFirstError := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, reconfigLogsAfterFirstError, test.ShouldEqual, 0)

	// Clear logs again to isolate just the retry attempts.
	logs.TakeAll()

	// Call updateRemotesAndRetryResourceConfigure multiple times.
	// Each call will attempt to retry configuring m_unrelated, which will fail again with the
	// invalid model. These repeated errors should NOT increment the clock (self-transitions
	// in Unhealthy state). Because the clock doesn't change, updateWeakAndOptionalDependents
	// should return early without additional reconfigurations of the foo component.
	for i := 0; i < 5; i++ {
		lr.(*localRobot).updateRemotesAndRetryResourceConfigure()
		// The clock should NOT increment - m_unrelated is still unusable.
		test.That(t, lr.(*localRobot).manager.resources.CurrLogicalClockValue(),
			test.ShouldEqual, clockAfterFirstError)
	}

	// Verify the foo component did not reconfigure during the retry attempts. The repeated
	// retry calls should NOT have caused additional reconfigurations.
	reconfigLogsAfterAllUpdates := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, reconfigLogsAfterAllUpdates, test.ShouldEqual, 0)

	// Verify that m_unrelated failed to build 5 times (one for each retry call).
	buildErrorLogs := logs.FilterMessageSnippet("resource build error: unknown resource type").Len()
	test.That(t, buildErrorLogs, test.ShouldEqual, 5)

	// Verify that no "could not get optional motor" logs were emitted during the retry
	// attempts, confirming the foo component wasn't repeatedly reconfigured due to the
	// unrelated resource's repeated errors.
	msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
	test.That(t, msgNum, test.ShouldEqual, 0)

	// Verify the key property: Clock incremented only once (for the first error),
	// not for each of the 5 retry attempts. Multiple calls to updateRemotesAndRetryResourceConfigure
	// with unchanged clock resulted in no additional reconfigurations.
	test.That(t, lr.(*localRobot).manager.resources.CurrLogicalClockValue(),
		test.ShouldEqual, clockAfterFirstError)

	// Verify that the foo component is still functional.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})
}

func TestOptionalDependencyUnrelatedResourceRemoval(t *testing.T) {
	// This test verifies behavior when an unrelated resource is removed from the config.
	// Removing any resource increments the clock, which causes
	// updateWeakAndOptionalDependents to run. Because the unrelated resource is not
	// part of any other resource's optional dep set, the per-resource snapshot is
	// unchanged and no reconfigure is triggered.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	// Register the optional child component.
	optionalChildModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	ocName := generic.Named("oc")
	resource.Register(
		generic.API,
		optionalChildModel,
		resource.Registration[*optionalChild, *optionalChildConfig]{
			Constructor: newOptionalChild,
		})
	defer resource.Deregister(generic.API, optionalChildModel)

	// Configure the robot with:
	// - An optional child "oc" that has required motor "m_required" and optional motor "m_optional"
	// - An unrelated motor "m_unrelated" that we will later remove
	cfg := config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m_required",
					OptionalMotor: "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_unrelated",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Get the optional child and verify initial state.
	ocRes, err := lr.ResourceByName(ocName)
	test.That(t, err, test.ShouldBeNil)
	oc, err := resource.AsType[*optionalChild](ocRes)
	test.That(t, err, test.ShouldBeNil)

	// The optional child reconfigured 1 or 2 times depending on construction order:
	// 1 if it saw m_optional during construction, 2 if it had to be reconfigured by
	// updateWeakAndOptionalDependents to pick up m_optional.
	initialReconfigCount := oc.reconfigCount
	test.That(t, initialReconfigCount, test.ShouldBeIn, []int{1, 2})

	// Verify both motors are accessible.
	test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
	test.That(t, oc.optionalMotor, test.ShouldNotBeNil)

	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Clear logs for cleaner assertions.
	logs.TakeAll()

	// Reconfigure to remove the unrelated motor.
	cfg = config.Config{
		Components: []resource.Config{
			{
				Name:  ocName.Name,
				API:   generic.API,
				Model: optionalChildModel,
				ConvertedAttributes: &optionalChildConfig{
					RequiredMotor: "m_required",
					OptionalMotor: "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Verify the clock incremented (m_unrelated was marked for removal).
	clockAfterRemoval := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, clockAfterRemoval, test.ShouldBeGreaterThan, initialClockValue)

	// Verify the optional child did NOT reconfigure. updateWeakAndOptionalDependents
	// ran because the clock advanced, but the optional child's resolved optional dep set
	// (m_optional) is unchanged so it was skipped.
	test.That(t, oc.reconfigCount, test.ShouldEqual, initialReconfigCount)

	// Verify both motors are still accessible.
	test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
	test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
}

func TestModularOptionalDependencyUnrelatedResourceRemoval(t *testing.T) {
	// This test is the modular version of TestOptionalDependencyUnrelatedResourceRemoval.
	// Removing any resource increments the clock, which causes
	// updateWeakAndOptionalDependents to run. Because the unrelated resource is not
	// part of any other resource's optional dep set, the per-resource snapshot is
	// unchanged and no reconfigure is triggered.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")

	// Configure the robot with:
	// - A foo component that has required motor "m_required" and optional motor "m_optional"
	// - An unrelated motor "m_unrelated" that we will later remove
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_unrelated",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)

	// Verify both motors are accessible through the foo component.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})

	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Clear logs for cleaner assertions.
	logs.TakeAll()

	// Reconfigure to remove the unrelated motor.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Verify the clock incremented (m_unrelated was marked for removal).
	clockAfterRemoval := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, clockAfterRemoval, test.ShouldBeGreaterThan, initialClockValue)

	// Verify the foo component did NOT reconfigure. updateWeakAndOptionalDependents
	// ran because the clock advanced, but foo's resolved optional dep set (m_optional)
	// is unchanged so it was skipped.
	reconfigLogsAfterRemoval := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, reconfigLogsAfterRemoval, test.ShouldEqual, 0)

	// Verify both motors are still accessible through the foo component after reconfiguration.
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

func TestModularOptionalDependencyModuleNameChange(t *testing.T) {
	// This test verifies behavior when an unrelated modular resource gets its module name changed.
	// When a module is renamed:
	//   1. The old module is shut down, marking its resources for removal (clock +1)
	//   2. The new module starts up and its resources are added (clock +1)
	// Each clock increment causes updateWeakAndOptionalDependents to run, but
	// foo's resolved optional dep set is unchanged across both increments so it
	// is never reconfigured.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")
	testModulePath := testutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")
	helperModel := resource.NewModel("rdk", "test", "helper")

	// Configure the robot with:
	// - A "foo" component from optional-deps module with optional dependencies on motors
	// - An unrelated "h_unrelated" helper component from the test module
	// The helper has no dependency relationship with foo, making it truly unrelated.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
			{
				Name:    "test",
				ExePath: testModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "h_unrelated",
				API:                 generic.API,
				Model:               helperModel,
				ConvertedAttributes: &resource.NoNativeConfig{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)

	// Verify both motors are accessible through the foo component.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})

	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Clear logs for cleaner assertions.
	logs.TakeAll()

	// Reconfigure with the module name changed from "test" to "test-renamed".
	// During this reconfiguration:
	//   1. The "test" module is shut down and h_unrelated is marked for removal
	//   2. The "test-renamed" module starts up and h_unrelated is rebuilt
	// Each step increments the clock, triggering foo to reconfigure twice.
	cfg = config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
			{
				Name:    "test-renamed",
				ExePath: testModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "h_unrelated",
				API:                 generic.API,
				Model:               helperModel,
				ConvertedAttributes: &resource.NoNativeConfig{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Verify the clock incremented. It should have incremented by 2:
	//   +1 when the old "test" module was removed
	//   +1 when the new "test-renamed" module's resources were added
	clockAfterModuleChange := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, clockAfterModuleChange, test.ShouldBeGreaterThan, initialClockValue)

	// Verify the foo component did NOT reconfigure. updateWeakAndOptionalDependents
	// ran on both clock increments, but foo's resolved optional dep set (m_optional)
	// is unchanged across both events so it was skipped each time.
	reconfigLogsAfterModuleChange := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, reconfigLogsAfterModuleChange, test.ShouldEqual, 0)

	// Verify both motors are still accessible through the foo component after reconfiguration.
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

func TestModularOptionalDependencyModuleCrash(t *testing.T) {
	// This test verifies behavior when an unrelated resource depends on a module that crashes.
	// The crash is simulated by moving the module binary (to block restarts) and then calling
	// DoCommand("kill_module") to trigger os.Exit(1) in the module process.
	//
	// During crash-and-retry: resources remain in retry state, so the logical clock does NOT
	// increment and foo does NOT reconfigure.
	// After recovery (binary restored): the clock increments when the module restarts and
	// h_unrelated is re-added. updateWeakAndOptionalDependents runs but foo's resolved
	// optional dep set is unchanged so it is not reconfigured.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")
	testModulePath := testutils.BuildTempModule(t, "module/testmodule")

	// Manually define models, as importing them can cause double registration.
	fooModel := resource.NewModel("acme", "demo", "foo")
	fooName := generic.Named("f")
	helperModel := resource.NewModel("rdk", "test", "helper")

	// Configure the robot with:
	// - A "foo" component from optional-deps module with optional dependencies on motors
	// - An unrelated "h_unrelated" helper component from the test module
	// The helper has no dependency relationship with foo, making it truly unrelated.
	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
			{
				Name:    "test",
				ExePath: testModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  fooName.Name,
				API:   generic.API,
				Model: fooModel,
				Attributes: rutils.AttributeMap{
					"required_motor": "m_required",
					"optional_motor": "m_optional",
				},
			},
			{
				Name:                "m_required",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "m_optional",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "h_unrelated",
				API:                 generic.API,
				Model:               helperModel,
				ConvertedAttributes: &resource.NoNativeConfig{},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg)

	// Assert that the foo component built successfully.
	fooRes, err := lr.ResourceByName(fooName)
	test.That(t, err, test.ShouldBeNil)

	// Assert that the helper component built successfully.
	helperRes, err := lr.ResourceByName(generic.Named("h_unrelated"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, helperRes, test.ShouldNotBeNil)

	// Verify both motors are accessible through the foo component.
	doCommandResp, err := fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})

	// Record the initial clock value before the crash.
	initialClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()

	// Clear logs; count only events from this point forward.
	logs.TakeAll()

	// Move the test module binary to prevent it from restarting after the crash.
	movedBinaryPath := testModulePath + ".moved"
	err = os.Rename(testModulePath, movedBinaryPath)
	test.That(t, err, test.ShouldBeNil)

	// Trigger a crash; the test module calls os.Exit(1) on this command.
	_, _ = helperRes.DoCommand(ctx, map[string]any{"command": "kill_module"})

	// Wait for the module manager to detect the crash and fail to restart (binary is missing).
	gotestutils.WaitForAssertion(t, func(tb testing.TB) {
		errorLogs := logs.FilterMessageSnippet("Error while restarting crashed module").Len()
		test.That(tb, errorLogs, test.ShouldBeGreaterThanOrEqualTo, 1)
	})

	// The helper stays in the resource graph during retry, but is unusable.
	helperRes, err = lr.ResourceByName(generic.Named("h_unrelated"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, helperRes, test.ShouldNotBeNil)
	_, err = helperRes.DoCommand(ctx, map[string]any{"command": "echo"})
	test.That(t, err, test.ShouldNotBeNil)

	// Verify both motors are still accessible through the foo component.
	// The foo component should continue working normally despite the unrelated module crash.
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})

	// Resources in retry state do not increment the clock, so foo does not reconfigure.
	finalClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, finalClockValue, test.ShouldEqual, initialClockValue)

	newReconfigCount := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, newReconfigCount, test.ShouldEqual, 0)

	// --- Recovery phase ---

	// Clear logs to count only recovery-phase events.
	logs.TakeAll()

	// Restore the binary so the module can restart successfully.
	err = os.Rename(movedBinaryPath, testModulePath)
	test.That(t, err, test.ShouldBeNil)

	// Drive restarts until the clock advances, indicating the module restarted
	// and h_unrelated was successfully re-added to the graph.
	//
	// The onUnexpectedExitHandler waits for 5s between restart attempts, so wait double that
	// amount of time.
	gotestutils.WaitForAssertionWithSleep(t, 500*time.Millisecond, 20, func(tb testing.TB) {
		lr.(*localRobot).updateRemotesAndRetryResourceConfigure()
		currentClock := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
		test.That(tb, currentClock, test.ShouldBeGreaterThan, initialClockValue)
	})

	recoveredClockValue := lr.(*localRobot).manager.resources.CurrLogicalClockValue()
	test.That(t, recoveredClockValue, test.ShouldBeGreaterThan, initialClockValue)

	// The clock increment does NOT trigger a reconfiguration of foo because its
	// resolved optional dep set is unchanged.
	recoveryReconfigCount := logs.FilterMessageSnippet("Reconfiguring resource for module").Len()
	test.That(t, recoveryReconfigCount, test.ShouldEqual, 0)

	// Both motors remain accessible through foo after the full crash-recovery cycle.
	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "required_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"required_motor_state": "moving: false"})

	doCommandResp, err = fooRes.DoCommand(ctx, map[string]any{"command": "optional_motor_state"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, doCommandResp, test.ShouldResemble, map[string]any{"optional_motor_state": "moving: false"})
}

// currentTargetID reports target's live instance id via a "probe":"id" DoCommand.
// Comparing the id across a reconfigure tells whether target was rebuilt (changed id)
// or not.
func currentTargetID(ctx context.Context, t *testing.T, lr robot.Robot, targetName resource.Name) any {
	targetRes, err := lr.ResourceByName(targetName)
	test.That(t, err, test.ShouldBeNil)
	resp, err := targetRes.DoCommand(ctx, map[string]any{"probe": "id"})
	test.That(t, err, test.ShouldBeNil)
	return resp["instance_id"]
}

func TestModularStalePointerAfterWeakOptionalRebuild(t *testing.T) {
	// Setup:
	//   - pointer-target (modular) declares an optional dep. When that dep's availability
	//     changes, the RDK's updateWeakAndOptionalDependents flow rebuilds the target by
	//     sending RemoveResource + AddResource to the module.
	//   - pointer-holder (modular, same module) explicitly depends_on the target and captures
	//     a direct Go pointer to it at construction time.
	//
	// Each target instance carries a unique ID. After the target is rebuilt, the holder must
	// be rebuilt too so its captured pointer references the current target instance — not the
	// one that was just closed. The test probes both the target directly and the
	// target-via-holder and asserts their instance IDs match. Without the cascade, the holder
	// ends up pointing at a closed prior instance and its proxied DoCommand either errors or
	// returns a stale ID.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	targetModel := resource.NewModel("acme", "demo", "pointer-target")
	holderModel := resource.NewModel("acme", "demo", "pointer-holder")
	targetName := generic.Named("target1")
	holderName := generic.Named("holder1")

	cfg := config.Config{
		Modules: []config.Module{
			{
				Name:    "optional-deps",
				ExePath: optionalDepsModulePath,
			},
		},
		Components: []resource.Config{
			{
				// The optional-dep target — its presence causes pointer-target to be
				// rebuilt by updateWeakAndOptionalDependents on every reconfigure cycle.
				Name:                "opt-dep",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:  targetName.Name,
				API:   generic.API,
				Model: targetModel,
				Attributes: rutils.AttributeMap{
					"optional_dep": "opt-dep",
				},
			},
			{
				// Explicit depends_on target1 via the config attribute; the module's
				// Validate returns it as a required dependency so the holder captures a Go
				// pointer to target1.
				Name:  holderName.Name,
				API:   generic.API,
				Model: holderModel,
				Attributes: rutils.AttributeMap{
					"target": targetName.Name,
				},
			},
		},
	}
	// Ensure fills in ImplicitOptionalDependsOn from Validate — required for
	// updateWeakAndOptionalDependents to consider pointer-target.
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)

	// assertHolderPointsToCurrentTarget fetches the current target instance ID directly,
	// then fetches the target instance ID via the holder (which proxies through its
	// captured pointer), and asserts they match. A mismatch means the holder is holding
	// a stale pointer to a previous target instance.
	assertHolderPointsToCurrentTarget := func(label string) {
		targetRes, err := lr.ResourceByName(targetName)
		test.That(t, err, test.ShouldBeNil)
		targetResp, err := targetRes.DoCommand(ctx, map[string]any{"probe": label})
		test.That(t, err, test.ShouldBeNil)

		holderRes, err := lr.ResourceByName(holderName)
		test.That(t, err, test.ShouldBeNil)
		holderResp, err := holderRes.DoCommand(ctx, map[string]any{"probe": label})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, holderResp["instance_id"], test.ShouldEqual, targetResp["instance_id"])
	}

	// Initial Reconfigure — builds target_v1 and holder_v1, then
	// updateWeakAndOptionalDependents rebuilds target (to target_v2) and the module's
	// cascade rebuilds holder (to holder_v2) so it points at target_v2.
	lr.Reconfigure(ctx, &cfg)
	assertHolderPointsToCurrentTarget("initial")
	targetIDInitial := currentTargetID(ctx, t, lr, targetName)

	// Add an unrelated motor to advance the clock and re-trigger
	// updateWeakAndOptionalDependents.
	cfg2 := cfg
	cfg2.Components = append(append([]resource.Config{}, cfg.Components...), resource.Config{
		Name:                "unrelated-motor",
		API:                 motor.API,
		Model:               fake.Model,
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, cfg2.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg2)
	// An unrelated change leaves target's optional-dep snapshot untouched, so
	// target must NOT be rebuilt — its instance is identical.
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldEqual, targetIDInitial)
	assertHolderPointsToCurrentTarget("after-unrelated-change")

	// Reconfigure target's opt-dep (Components[0]) in place. opt-dep stays present,
	// but its config diff makes it reconfigure and bump its graph clock. That stales
	// target's recorded  optional-dep snapshot, so updateWeakAndOptionalDependents
	// rebuilds target to a fresh instance, and the module cascade must rebuild holder
	// to point at it.
	targetIDBefore := currentTargetID(ctx, t, lr, targetName)

	cfg3 := cfg2
	cfg3.Components = append([]resource.Config{}, cfg2.Components...)
	cfg3.Components[0].Attributes = rutils.AttributeMap{"version": 1}
	test.That(t, cfg3.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg3)

	// target rebuilt to a new instance because its optional dependency's clock advanced.
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldNotEqual, targetIDBefore)

	// holder was cascaded along with that rebuild, so it points at the new target instance
	// rather than the closed prior one.
	assertHolderPointsToCurrentTarget("after-optdep-change")
}

func TestModularStalePointerCascadeFanOut(t *testing.T) {
	// Verifies that a single rebuild of a resource with multiple explicit dependents
	// cascades to rebuild every one of them.
	// Setup:
	//   - One pointer-target with an optional dependency (triggers updateWeakAndOptionalDependents
	//     on every reconfigure).
	//   - Three pointer-holders, each depending on that target.
	//
	// After reconfigure, every holder should route DoCommand through the current target
	// instance. If the cascade only rebuilds one holder (or rebuilds them against different
	// target instances), some holder's instance_id would diverge from the others or from the
	// target itself.

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	targetModel := resource.NewModel("acme", "demo", "pointer-target")
	holderModel := resource.NewModel("acme", "demo", "pointer-holder")
	targetName := generic.Named("target")
	holderNames := []resource.Name{
		generic.Named("holder-a"),
		generic.Named("holder-b"),
		generic.Named("holder-c"),
	}

	components := []resource.Config{
		{
			Name:                "opt-dep",
			API:                 motor.API,
			Model:               fake.Model,
			ConvertedAttributes: &fake.Config{},
		},
		{
			Name:  targetName.Name,
			API:   generic.API,
			Model: targetModel,
			Attributes: rutils.AttributeMap{
				"optional_dep": "opt-dep",
			},
		},
	}
	for _, hn := range holderNames {
		components = append(components, resource.Config{
			Name:  hn.Name,
			API:   generic.API,
			Model: holderModel,
			Attributes: rutils.AttributeMap{
				"target": targetName.Name,
			},
		})
	}

	cfg := config.Config{
		Modules: []config.Module{
			{Name: "optional-deps", ExePath: optionalDepsModulePath},
		},
		Components: components,
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)

	assertAllHoldersPointToCurrentTarget := func(label string) {
		targetRes, err := lr.ResourceByName(targetName)
		test.That(t, err, test.ShouldBeNil)
		targetResp, err := targetRes.DoCommand(ctx, map[string]any{"probe": label})
		test.That(t, err, test.ShouldBeNil)
		wantID := targetResp["instance_id"]

		for _, hn := range holderNames {
			holderRes, err := lr.ResourceByName(hn)
			test.That(t, err, test.ShouldBeNil)
			holderResp, err := holderRes.DoCommand(ctx, map[string]any{"probe": label})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, holderResp["instance_id"], test.ShouldEqual, wantID)
		}
	}
	lr.Reconfigure(ctx, &cfg)
	assertAllHoldersPointToCurrentTarget("initial")
	targetIDInitial := currentTargetID(ctx, t, lr, targetName)

	// Push an unrelated config change to advance the clock and re-trigger
	// updateWeakAndOptionalDependents.
	cfg2 := cfg
	cfg2.Components = append(append([]resource.Config{}, cfg.Components...), resource.Config{
		Name:                "unrelated-motor",
		API:                 motor.API,
		Model:               fake.Model,
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, cfg2.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg2)
	// Skip held: the unrelated change leaves target's optional-dep snapshot untouched, so
	// target is not rebuilt and its instance is identical. (Without this, the holder==target
	// checks below pass even if the skip regressed.)
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldEqual, targetIDInitial)
	assertAllHoldersPointToCurrentTarget("after-unrelated-change")

	// Force an actual rebuild: change target's optional dependency (opt-dep, Components[0])
	// in place so its clock bumps, staling target's snapshot and rebuilding target to a new
	// instance. Capturing the id beforehand lets us assert the rebuild really fired.
	targetIDBefore := currentTargetID(ctx, t, lr, targetName)

	cfg3 := cfg2
	cfg3.Components = append([]resource.Config{}, cfg2.Components...)
	cfg3.Components[0].Attributes = rutils.AttributeMap{"version": 1}
	test.That(t, cfg3.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg3)

	// target rebuilt to a new instance because its optional dependency's clock advanced.
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldNotEqual, targetIDBefore)

	// The fan-out is now meaningfully exercised: all three holders must have cascaded to
	// the new target instance.
	assertAllHoldersPointToCurrentTarget("after-optdep-change")
}

func TestModularStalePointerCascadeChain(t *testing.T) {
	// Verifies that cascades propagate transitively through multi-hop explicit-dep chains.
	// Exercises the recursive cascade inside rebuildResourceWithVisited (invoked by the outer
	// addResource cascade), not just the single-level cascade in addResource.
	// Setup:  target → mid-holder (depends on target) → top-holder (depends on mid-holder)
	//
	// When target is rebuilt:
	//  1. addResource(target_new) cascade finds mid-holder in internalDeps[target], rebuilds it.
	//  2. rebuildResourceWithVisited(mid-holder) runs ITS cascade over internalDeps[mid-holder],
	//     rebuilds top-holder.
	//  3. top-holder ends up freshly constructed, its captured pointer referencing the freshly
	//     constructed mid-holder, which references the new target.
	//
	// top-holder.DoCommand proxies → mid-holder.DoCommand → target.DoCommand → returns the
	// current target's instance_id. Any break in the chain would either error or report a
	// stale ID.

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger, WithDisableCompleteConfigWorker())

	optionalDepsModulePath := testutils.BuildTempModule(t, "examples/customresources/demos/optionaldepsmodule")

	targetModel := resource.NewModel("acme", "demo", "pointer-target")
	holderModel := resource.NewModel("acme", "demo", "pointer-holder")
	targetName := generic.Named("target")
	midName := generic.Named("mid-holder")
	topName := generic.Named("top-holder")

	cfg := config.Config{
		Modules: []config.Module{
			{Name: "optional-deps", ExePath: optionalDepsModulePath},
		},
		Components: []resource.Config{
			{
				Name:                "opt-dep",
				API:                 motor.API,
				Model:               fake.Model,
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:  targetName.Name,
				API:   generic.API,
				Model: targetModel,
				Attributes: rutils.AttributeMap{
					"optional_dep": "opt-dep",
				},
			},
			{
				Name:  midName.Name,
				API:   generic.API,
				Model: holderModel,
				Attributes: rutils.AttributeMap{
					"target": targetName.Name,
				},
			},
			{
				Name:  topName.Name,
				API:   generic.API,
				Model: holderModel,
				Attributes: rutils.AttributeMap{
					"target": midName.Name,
				},
			},
		},
	}
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)

	// The top-holder proxies DoCommand through mid-holder, which proxies through target.
	// If any link in the chain holds a stale pointer, the ID returned at the top won't
	// match the target's current ID.
	assertTopHolderReachesCurrentTarget := func(label string) {
		targetRes, err := lr.ResourceByName(targetName)
		test.That(t, err, test.ShouldBeNil)
		targetResp, err := targetRes.DoCommand(ctx, map[string]any{"probe": label})
		test.That(t, err, test.ShouldBeNil)

		topRes, err := lr.ResourceByName(topName)
		test.That(t, err, test.ShouldBeNil)
		topResp, err := topRes.DoCommand(ctx, map[string]any{"probe": label})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, topResp["instance_id"], test.ShouldEqual, targetResp["instance_id"])
	}

	lr.Reconfigure(ctx, &cfg)
	assertTopHolderReachesCurrentTarget("initial")
	targetIDInitial := currentTargetID(ctx, t, lr, targetName)

	// Push an unrelated config change to advance the clock and re-trigger the cascade path.
	cfg2 := cfg
	cfg2.Components = append(append([]resource.Config{}, cfg.Components...), resource.Config{
		Name:                "unrelated-motor",
		API:                 motor.API,
		Model:               fake.Model,
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, cfg2.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg2)
	// The unrelated change leaves target's optional-dep snapshot untouched, so
	// target is not rebuilt and its instance is identical.
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldEqual, targetIDInitial)
	assertTopHolderReachesCurrentTarget("after-unrelated-change")

	// Force an actual rebuild: change target's opt-dep (Components[0]) in place so
	// its clock bumps, staling target's snapshot and rebuilding target to a new
	// instance.
	targetIDBefore := currentTargetID(ctx, t, lr, targetName)

	cfg3 := cfg2
	cfg3.Components = append([]resource.Config{}, cfg2.Components...)
	cfg3.Components[0].Attributes = rutils.AttributeMap{"version": 1}
	test.That(t, cfg3.Ensure(false, logger), test.ShouldBeNil)
	lr.Reconfigure(ctx, &cfg3)

	// target rebuilt to a new instance because its optional dependency's clock advanced.
	test.That(t, currentTargetID(ctx, t, lr, targetName), test.ShouldNotEqual, targetIDBefore)

	// The transitive cascade is now meaningfully exercised: top-holder must reach the new
	// target instance through the freshly rebuilt mid-holder.
	assertTopHolderReachesCurrentTarget("after-optdep-change")
}

func TestWeakOptionalDepClocksEqual(t *testing.T) {
	// Unit tests for the snapshot comparison that gates the weak/optional reconfigure skip.
	// A nil snapshot means "not yet recorded" and must never compare equal (so a freshly
	// built resource is always considered for a follow-up update); two non-nil snapshots
	// are equal iff they have identical key sets and identical clock values.
	m := motor.Named("m")
	m1 := motor.Named("m1")

	for _, tc := range []struct {
		name string
		a, b map[resource.Name]int64
		want bool
	}{
		{
			name: "both nil are not equal",
			a:    nil,
			b:    nil,
			want: false,
		},
		{
			name: "nil vs empty is not equal",
			a:    nil,
			b:    map[resource.Name]int64{},
			want: false,
		},
		{
			name: "two empty non-nil snapshots are equal",
			a:    map[resource.Name]int64{},
			b:    map[resource.Name]int64{},
			want: true,
		},
		{
			name: "same single key and value is equal",
			a:    map[resource.Name]int64{m: 1},
			b:    map[resource.Name]int64{m: 1},
			want: true,
		},
		{
			name: "same key, different clock value is not equal",
			a:    map[resource.Name]int64{m: 1},
			b:    map[resource.Name]int64{m: 2},
			want: false,
		},
		{
			name: "same length, different key is not equal",
			a:    map[resource.Name]int64{m: 1},
			b:    map[resource.Name]int64{m1: 1},
			want: false,
		},
		{
			name: "differing length (extra key) is not equal",
			a:    map[resource.Name]int64{m: 1},
			b:    map[resource.Name]int64{m: 1, m1: 2},
			want: false,
		},
		{
			name: "multiple identical keys and values is equal",
			a:    map[resource.Name]int64{m: 1, m1: 2},
			b:    map[resource.Name]int64{m: 1, m1: 2},
			want: true,
		},
		{
			name: "multiple keys, one clock value differs is not equal",
			a:    map[resource.Name]int64{m: 1, m1: 2},
			b:    map[resource.Name]int64{m: 1, m1: 3},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, weakOptionalDepClocksEqual(tc.a, tc.b), test.ShouldEqual, tc.want)
			// The comparison must be symmetric: swapping the arguments must not change the
			// result.
			test.That(t, weakOptionalDepClocksEqual(tc.b, tc.a), test.ShouldEqual, tc.want)
		})
	}
}
