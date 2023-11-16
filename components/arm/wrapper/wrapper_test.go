package wrapper

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	cfg := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "../universalrobots/ur5e.json",
			ArmName:       "does not exist0",
		},
	}

	armName := arm.Named("foo")
	cfg1 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "../xarm/xarm6_kinematics.json",
			ArmName:       armName.ShortName(),
		},
	}

	cfg1Err := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "../xarm/xarm6_kinematics.json",
			ArmName:       "dne1",
		},
	}

	cfg2Err := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "DNE",
			ArmName:       armName.ShortName(),
		},
	}

	conf, err := resource.NativeConfig[*Config](cfg)
	test.That(t, err, test.ShouldBeNil)

	model, err := modelFromPath(conf.ModelFilePath, cfg.Name)
	test.That(t, err, test.ShouldBeNil)

	actualArm := &inject.Arm{}

	wrapperArm := &Arm{
		Named:  cfg.ResourceName().AsNamed(),
		model:  model,
		actual: &inject.Arm{},
		logger: logger,
	}

	deps := resource.Dependencies{armName: actualArm}

	test.That(t, wrapperArm.Reconfigure(context.Background(), deps, cfg1), test.ShouldBeNil)

	err = wrapperArm.Reconfigure(context.Background(), deps, cfg1Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "missing from dep")

	err = wrapperArm.Reconfigure(context.Background(), deps, cfg2Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only files")
}
