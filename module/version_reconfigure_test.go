package module_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/encoder"
	fakeencoder "go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/utils"
)

func TestValidationFailureDuringReconfiguration(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:     "AcmeModule",
				ExePath:  "multiversionmodule/run_version1.sh",
				LogLevel: "debug",
			},
		},
		Components: []resource.Config{
			{
				Name:                "generic1",
				Model:               resource.NewModel("acme", "demo", "multiversionmodule"),
				API:                 generic.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "motor1",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "encoder1",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 encoder.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakeencoder.Config{},
			},
		},
	}

	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer robot.Close(ctx)

	// Assert that generic1 was added.
	_, err = robot.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that there were no validation or component building errors
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Read the config, swap to `run_version2.sh`, and overwrite the config, triggering a
	// reconfigure where `generic1` will fail validation.
	cfg.Modules[0].ExePath = utils.ResolveFile("module/multiversionmodule/run_version2.sh")
	robot.Reconfigure(ctx, cfg)

	// Check that generic1 now has a config validation error.
	_, err = robot.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring,
		`config validation error found in modular resource: rdk:component:generic/generic1:`)
	test.That(t, err.Error(), test.ShouldContainSubstring,
		`version 2 requires a parameter`)

	// Assert that Validation failure message is present
	//
	// Race condition safety: Resource removal should occur after modular resource validation (during completeConfig), so if
	// ResourceByName is failing, these errors should already be present
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 1)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)
}

func TestVersionBumpWithNewImplicitDeps(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:     "AcmeModule",
				ExePath:  "multiversionmodule/run_version1.sh",
				LogLevel: "debug",
			},
		},
		Components: []resource.Config{
			{
				Name:                "generic1",
				Model:               resource.NewModel("acme", "demo", "multiversionmodule"),
				API:                 generic.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "motor1",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 motor.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakemotor.Config{},
			},
			{
				Name:                "encoder1",
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				API:                 encoder.API,
				Attributes:          utils.AttributeMap{},
				ConvertedAttributes: &fakeencoder.Config{},
			},
		},
	}

	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer robot.Close(ctx)

	// Assert that generic1 was added.
	_, err = robot.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that there were no validation or component building errors
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Swap in `run_version3.sh`. Version 3 requires `generic1` to have a `motor` in its
	// attributes. This config change should result in `generic1` becoming unavailable.
	cfg.Modules[0].ExePath = utils.ResolveFile("module/multiversionmodule/run_version3.sh")
	robot.Reconfigure(ctx, cfg)

	_, err = robot.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `resource "rdk:component:generic/generic1" not available`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `version 3 requires a motor`)

	// Assert that Validation failure message is present
	//
	// Race condition safety: Resource removal should occur after modular resource validation (during completeConfig), so if
	// ResourceByName is failing, these errors should already be present
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 1)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Update the generic1 configuration to have a `motor` attribute. The following reconfiguration
	// round should make the `generic1` component available again.
	for i, c := range cfg.Components {
		if c.Name == "generic1" {
			cfg.Components[i].Attributes = utils.AttributeMap{"motor": "motor1"}
		}
	}
	robot.Reconfigure(ctx, cfg)
	_, err = robot.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldBeNil)
}
