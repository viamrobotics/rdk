package fake

import (
	"context"
	"math"
	"testing"

	"go.viam.com/test"

	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

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

func TestGet3DModels(t *testing.T) {
	ctx := context.Background()
	confNo3DModels := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: xArm7Model,
		},
	}
	a, err := NewArm(ctx, nil, confNo3DModels, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	fakeArm, _ := a.(*Arm)
	models, err := fakeArm.Get3DModels(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(models), test.ShouldEqual, 0)

	confWith3DModels := resource.Config{
		Name: "testArm",
		ConvertedAttributes: &Config{
			ArmModel: ur5eModel,
		},
	}
	err = fakeArm.reconfigure(ctx, nil, confWith3DModels)
	test.That(t, err, test.ShouldBeNil)
	models, err = fakeArm.Get3DModels(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(models), test.ShouldEqual, 7)
	test.That(t, models["ee_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "ee_link").Mesh)
	test.That(t, models["ee_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["forearm_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "forearm_link").Mesh)
	test.That(t, models["forearm_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["upper_arm_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "upper_arm_link").Mesh)
	test.That(t, models["upper_arm_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["wrist_1_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "wrist_1_link").Mesh)
	test.That(t, models["wrist_1_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["wrist_2_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "wrist_2_link").Mesh)
	test.That(t, models["wrist_2_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["base_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "base_link").Mesh)
	test.That(t, models["base_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
	test.That(t, models["shoulder_link"].Mesh, test.ShouldResemble, models3d.ThreeDMeshFromName("ur5e", "shoulder_link").Mesh)
	test.That(t, models["shoulder_link"].ContentType, test.ShouldResemble, "model/gltf-binary")
}
