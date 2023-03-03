package wrapper

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestUpdateAction(t *testing.T) {
	logger := golog.NewTestLogger(t)

	cfg := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ModelFilePath: "../universalrobots/ur5e.json",
			ArmName:       "does not exist0",
		},
	}

	shouldNotReconfigureCfg := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ModelFilePath: "../xarm/xarm6_kinematics.json",
		},
	}

	shouldReconfigureCfg := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ArmName: "does not exist1",
		},
	}

	shouldErr := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ModelFilePath: "DNE",
		},
	}

	attrs, ok := cfg.ConvertedAttributes.(*AttrConfig)
	test.That(t, ok, test.ShouldBeTrue)

	model, err := referenceframe.ModelFromPath(attrs.ModelFilePath, cfg.Name)
	test.That(t, err, test.ShouldBeNil)

	wrapperArm := &Arm{
		Name:   cfg.Name,
		model:  model,
		actual: &inject.Arm{},
		logger: logger,
		robot:  &inject.Robot{},
	}

	// scenario where we do not reconfigure
	test.That(t, wrapperArm.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we reconfigure
	test.That(t, wrapperArm.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we error out
	test.That(t, func() { wrapperArm.UpdateAction(&shouldErr) }, test.ShouldPanic)

	// wrap with reconfigurable arm to test the codepath that will be executed during reconfigure
	reconfArm, err := arm.WrapWithReconfigurable(wrapperArm, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	// scenario where we do not reconfigure
	obj, canUpdate := reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we reconfigure
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we error out
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, func() { obj.UpdateAction(&shouldErr) }, test.ShouldPanic)
}
