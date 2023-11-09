package fake

import (
	"context"
	"testing"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

func TestReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)

	cfg := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: "ur5e",
		},
	}

	conf1 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: "xArm6",
		},
	}

	conf2 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "fake_model.json",
		},
	}

	conf1Err := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: "DNE",
		},
	}

	conf2Err := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "DNE",
		},
	}

	conf, err := resource.NativeConfig[*Config](cfg)
	test.That(t, err, test.ShouldBeNil)

	model, err := modelFromName(conf.ArmModel, cfg.Name)
	test.That(t, err, test.ShouldBeNil)

	fakeArm := &Arm{
		Named:  cfg.ResourceName().AsNamed(),
		joints: &pb.JointPositions{Values: make([]float64, len(model.DoF()))},
		model:  model,
		logger: logger,
	}

	test.That(t, fakeArm.Reconfigure(context.Background(), nil, conf1), test.ShouldBeNil)
	model, err = modelFromName(conf1.ConvertedAttributes.(*Config).ArmModel, cfg.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm.joints.Values, test.ShouldResemble, make([]float64, len(model.DoF())))
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	test.That(t, fakeArm.Reconfigure(context.Background(), nil, conf2), test.ShouldBeNil)
	model, err = referenceframe.ModelFromPath(conf2.ConvertedAttributes.(*Config).ModelFilePath, cfg.Name)
	test.That(t, err, test.ShouldBeNil)
	modelJoints := make([]float64, len(model.DoF()))
	test.That(t, fakeArm.joints.Values, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(context.Background(), nil, conf1Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported")
	test.That(t, fakeArm.joints.Values, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(context.Background(), nil, conf2Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only files")
	test.That(t, fakeArm.joints.Values, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)
}
