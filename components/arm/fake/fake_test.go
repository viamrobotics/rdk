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
	logger := golog.NewLogger("test")

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

	shouldErr := config.Component{
		Name: "testArm",
		ConvertedAttributes: &AttrConfig{
			ArmModel: "DNE",
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

	// scenario where we error out
	test.That(t, fakeArm.UpdateAction(&shouldErr), test.ShouldBeError)

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

	// scenario where we error out
	obj, canUpdate = reconfArm.(config.ComponentUpdate)
	test.That(t, canUpdate, test.ShouldBeTrue)
	test.That(t, obj.UpdateAction(&shouldErr), test.ShouldBeError)
}
