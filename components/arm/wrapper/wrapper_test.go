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
	logger, logs := golog.NewObservedTestLogger(t)

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

	shouldLogErr := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ModelFilePath: "DNE",
		},
	}

	type foo struct {
		daboDee string
	}

	shouldRebuild := config.Component{
		Name: "testArm",
		ConvertedAttributes: &foo{
			daboDee: "DNE",
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
	test.That(t, wrapperArm.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.Reconfigure)

	// scenario where we log err
	test.That(t, wrapperArm.UpdateAction(&shouldLogErr), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 1)

	// scenario where we rebuild
	test.That(t, wrapperArm.UpdateAction(&shouldRebuild), test.ShouldEqual, config.Rebuild)

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
	test.That(t, obj.UpdateAction(&shouldReconfigureCfg), test.ShouldEqual, config.Reconfigure)

	// scenario where we log err
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldLogErr), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 2)

	// scenario where we rebuild
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldRebuild), test.ShouldEqual, config.Rebuild)
}
