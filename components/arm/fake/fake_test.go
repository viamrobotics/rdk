package fake

import (
	"testing"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

func TestUpdateAction(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)

	cfg := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ArmModel: "ur5e",
		},
	}

	shouldNotReconfigureCfg := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ArmModel: "xArm6",
		},
	}

	shouldNotReconfigureCfgAgain := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ModelFilePath: "fake_model.json",
		},
	}

	shouldLogErr := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ArmModel: "DNE",
		},
	}

	shouldLogErrAgain := config.Component{
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
			daboDee: "string",
		},
	}

	attrs, ok := cfg.ConvertedAttributes.(*AttrConfig)
	test.That(t, ok, test.ShouldBeTrue)

	model, err := modelFromName(attrs.ArmModel, cfg.Name)
	test.That(t, err, test.ShouldBeNil)

	fakeArm := &Arm{
		Name:   cfg.Name,
		joints: &pb.JointPositions{Values: make([]float64, len(model.DoF()))},
		model:  model,
		logger: logger,
	}

	// scenario where we do not reconfigure
	test.That(t, fakeArm.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we do not reconfigure again
	test.That(t, fakeArm.UpdateAction(&shouldNotReconfigureCfgAgain), test.ShouldEqual, config.None)

	// scenario where we log error only
	test.That(t, fakeArm.UpdateAction(&shouldLogErr), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 1)

	// scenario where we log error only
	test.That(t, fakeArm.UpdateAction(&shouldLogErrAgain), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 2)

	// scenario where we rebuild
	test.That(t, fakeArm.UpdateAction(&shouldRebuild), test.ShouldEqual, config.Rebuild)

	// wrap with reconfigurable arm to test the codepath that will be executed during reconfigure
	reconfArm, err := arm.WrapWithReconfigurable(fakeArm, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	// scenario where we do not reconfigure
	obj, canUpdate := reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldNotReconfigureCfg), test.ShouldEqual, config.None)

	// scenario where we do not reconfigure again
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldNotReconfigureCfgAgain), test.ShouldEqual, config.None)

	// scenario where we log error
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldLogErr), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 3)

	// scenario where we log error again
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldLogErrAgain), test.ShouldEqual, config.None)
	test.That(t, len(logs.All()), test.ShouldEqual, 4)

	// scenario where we rebuild
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldRebuild), test.ShouldEqual, config.Rebuild)
}
