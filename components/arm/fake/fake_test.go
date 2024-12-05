package fake

import (
	"context"
	"math"
	"testing"

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
			ModelFilePath: "../example_kinematics/xarm6_kinematics_test.json",
		},
	}

	conf2 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "fake_model.json",
		},
	}

	conf3 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "zero_model.json",
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
		joints: referenceframe.FloatsToInputs(make([]float64, len(model.DoF()))),
		model:  model,
		logger: logger,
	}

	test.That(t, fakeArm.Reconfigure(context.Background(), nil, conf1), test.ShouldBeNil)
	model, err = referenceframe.ParseModelJSONFile(conf1.ConvertedAttributes.(*Config).ModelFilePath, cfg.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm.joints, test.ShouldResemble, make([]referenceframe.Input, len(model.DoF())))
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	test.That(t, fakeArm.Reconfigure(context.Background(), nil, conf2), test.ShouldBeNil)
	model, err = referenceframe.ParseModelJSONFile(conf2.ConvertedAttributes.(*Config).ModelFilePath, cfg.Name)
	test.That(t, err, test.ShouldBeNil)
	modelJoints := make([]referenceframe.Input, len(model.DoF()))
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(context.Background(), nil, conf1Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported")
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(context.Background(), nil, conf2Err)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only files")
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(context.Background(), nil, conf3)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "fake arm built with zero degrees-of-freedom")
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)
}

func TestJointPositions(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: "ur5e",
		},
	}

	// Round trip test for MoveToJointPositions -> JointPositions
	arm, err := NewArm(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	samplePositions := []referenceframe.Input{{0}, {math.Pi}, {-math.Pi}, {0}, {math.Pi}, {-math.Pi}}
	test.That(t, arm.MoveToJointPositions(ctx, samplePositions, nil), test.ShouldBeNil)
	positions, err := arm.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(positions), test.ShouldEqual, len(samplePositions))
	for i := range samplePositions {
		test.That(t, positions[i], test.ShouldResemble, samplePositions[i])
	}

	// Round trip test for GoToInputs -> CurrentInputs
	sampleInputs := make([]referenceframe.Input, len(arm.ModelFrame().DoF()))
	test.That(t, arm.GoToInputs(ctx, sampleInputs), test.ShouldBeNil)
	inputs, err := arm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sampleInputs, test.ShouldResemble, inputs)
}
