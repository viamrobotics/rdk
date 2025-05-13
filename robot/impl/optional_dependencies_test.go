package robotimpl

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
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

	oc.requiredMotor, err = motor.FromDependencies(deps, optionalChildConfig.RequiredMotor)
	if err != nil {
		return fmt.Errorf("could not get required motor %s from dependencies",
			optionalChildConfig.RequiredMotor)
	}

	oc.optionalMotor, err = motor.FromDependencies(deps, optionalChildConfig.OptionalMotor)
	if err != nil {
		oc.logger.Infof("could not get optional motor %s from dependencies; continuing",
			optionalChildConfig.OptionalMotor)
	}

	return nil
}

func TestNonModularOptionalDependencies(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger)

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
		// from dependencies _twice_. The first of both is from construction (invokes
		// `Reconfigure`) of the resource, and the second of both is from reconfiguring of the
		// resource due to an unconditional call to `updateWeakAndOptionalDependents` directly
		// after `completeConfig`.
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 2)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

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

		// Assert that the optional child has reconfigured _three_ times. Two from the
		// previous construction and one from the reconfigure to pass 'm1' in _addition_ to 'm'
		// as a dependency. Assert that there were no more logs (still 2) about failures to
		// "get optional motor."
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 3)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

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

		// Assert that the optional child has reconfigured four times. Three from the previous
		// construction and reconfigure, and one from the most recent reconfigure to pass
		// _only_ 'm' as a dependency (no 'm1'). Assert that there was another log (now 3)
		// about failures to "get optional motor."
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 4)
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 3)

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

		// Assert that the optional child reconfigured twice. The first is from construction
		// (invokes `Reconfigure`) of the resource, and the second is from reconfiguring of
		// the resource due to an unconditional call to `updateWeakAndOptionalDependents`
		// directly after `completeConfig`.
		oc, err := resource.AsType[*optionalChild](ocRes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, oc.reconfigCount, test.ShouldEqual, 2)

		// Assert that there are either 3 (no new) _or_ 4 logs about an inability to "get
		// optional motor."
		//
		// The optional child _might_ get 'm1' as a dependency as part of its initial
		// construction, in which case no log will be emitted, or it _might_ get 'm1' as a
		// dependency as part of the reconfigure triggered by the unconditional call to
		// `updateWeakAndOptionalDependents`, in which case one log will emitted due to the
		// initial destruction lacking the 'm1' dependency.
		//
		// Optional dependencies are _not_ represented as edges in the resource graph and have
		// no influence on build order. 3 logs would mean the order was m -> m1 -> oc or m1 ->
		// m -> oc. 4 logs would mean the order was m -> oc -> m1.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldBeIn, []int{3, 4})

		// Assert that, on the component itself, `requiredMotor` and `optionalMotor` are now
		// set.
		test.That(t, oc.requiredMotor, test.ShouldNotBeNil)
		test.That(t, oc.optionalMotor, test.ShouldNotBeNil)
	}
}

func TestModularOptionalDependencies(t *testing.T) {
	// A copy of TestNonModularOptionalDependencies with a modular component instead of a
	// resource defined in this file.

	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	lr := setupLocalRobot(t, ctx, &config.Config{}, logger)

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
		// _twice_. The first is from construction (invokes `Reconfigure`) of the resource,
		// and the second is from reconfiguring of the resource due to an unconditional call
		// to `updateWeakAndOptionalDependents` directly after `completeConfig`.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

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

		// Assert that there were no more logs (still 2) about failures to "get optional
		// motor."
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 2)

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

		// Assert that there was another log (still 3) about a failure to "get optional
		// motor."
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldEqual, 3)

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

		// Assert that there are either 3 (no new) _or_ 4 logs about an inability to "get
		// optional motor."
		//
		// The foo component child _might_ get 'm1' as a dependency as part of its initial
		// construction, in which case no log will be emitted, or it _might_ get 'm1' as a
		// dependency as part of the reconfigure triggered by the unconditional call to
		// `updateWeakAndOptionalDependents`, in which case one log will emitted due to the
		// initial destruction lacking the 'm1' dependency.
		//
		// Optional dependencies are _not_ represented as edges in the resource graph and have
		// no influence on build order. 3 logs would mean the order was m -> m1 -> f or m1 ->
		// m -> f. 4 logs would mean the order was m -> f -> m1.
		msgNum := logs.FilterMessageSnippet("could not get optional motor").Len()
		test.That(t, msgNum, test.ShouldBeIn, []int{3, 4})

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
