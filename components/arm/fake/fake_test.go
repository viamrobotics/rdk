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
	ctx := context.Background()
	conf0 := resource.Config{
		Name:                "testArm",
		ConvertedAttributes: &Config{},
	}

	conf1 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: xArm6Model,
		},
	}

	conf2 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "zero_model.json",
		},
	}

	conf3 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "DNE",
		},
	}

	conf4 := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ModelFilePath: "a.json",
			ArmModel:      ur5eModel,
		},
	}

	a, err := NewArm(ctx, nil, conf0, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	fakeArm, _ := a.(*Arm)
	test.That(t, fakeArm.armModel, test.ShouldResemble, "")

	model, err := fakeArm.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm.joints, test.ShouldResemble, make([]referenceframe.Input, len(model.DoF())))
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	test.That(t, fakeArm.Reconfigure(ctx, nil, conf1), test.ShouldBeNil)
	model, err = fakeArm.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	modelJoints := make([]referenceframe.Input, len(model.DoF()))
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)
	test.That(t, fakeArm.armModel, test.ShouldResemble, xArm6Model)

	err = fakeArm.Reconfigure(ctx, nil, conf2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "fake arm built with zero degrees-of-freedom")
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(ctx, nil, conf3)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "only files")
	test.That(t, fakeArm.joints, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)

	err = fakeArm.Reconfigure(ctx, nil, conf4)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldResemble, errAttrCfgPopulation)
}

func TestJointPositions(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: ur5eModel,
		},
	}

	// Round trip test for MoveToJointPositions -> JointPositions
	arm, err := NewArm(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	samplePositions := []referenceframe.Input{0, math.Pi, -math.Pi, 0, math.Pi, -math.Pi}
	test.That(t, arm.MoveToJointPositions(ctx, samplePositions, nil), test.ShouldBeNil)
	positions, err := arm.JointPositions(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(positions), test.ShouldEqual, len(samplePositions))
	for i := range samplePositions {
		test.That(t, positions[i], test.ShouldResemble, samplePositions[i])
	}
	m, err := arm.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Round trip test for GoToInputs -> CurrentInputs
	sampleInputs := make([]referenceframe.Input, len(m.DoF()))
	test.That(t, arm.GoToInputs(ctx, sampleInputs), test.ShouldBeNil)
	inputs, err := arm.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sampleInputs, test.ShouldResemble, inputs)
}
