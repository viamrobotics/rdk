package fake

import (
	"context"
	"math"
	"testing"

	"go.viam.com/test"

	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/components/arm/kinematics"
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
			ArmModel: kinematics.UR5e,
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
			ArmModel: kinematics.XArm7,
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
			ArmModel: kinematics.UR5e,
		},
	}
	err = fakeArm.reconfigure(ctx, nil, confWith3DModels)
	test.That(t, err, test.ShouldBeNil)
	models, err = fakeArm.Get3DModels(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	want := models3d.Visuals("ur5e")
	test.That(t, len(models), test.ShouldEqual, 7)
	for link, glb := range want {
		test.That(t, models[link].GetMesh(), test.ShouldResemble, glb)
		test.That(t, models[link].GetContentType(), test.ShouldEqual, "model/gltf-binary")
	}

	// the typed model carries the same meshes, so a client reading it sees what Get3DModels serves
	model, err := fakeArm.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	pb, err := referenceframe.ModelToProto(model)
	test.That(t, err, test.ShouldBeNil)
	for _, link := range pb.GetLinks() {
		glb, ok := want[link.GetId()]
		if !ok {
			test.That(t, link.GetVisual(), test.ShouldBeEmpty)
			continue
		}
		test.That(t, link.GetVisual(), test.ShouldHaveLength, 1)
		test.That(t, link.GetVisual()[0].GetMesh().GetMesh(), test.ShouldResemble, glb)
	}

	// a model read from a file carries whatever the file holds, which for the SVA files is nothing
	confFromFile := resource.Config{
		Name:                "testArm",
		ConvertedAttributes: &Config{ModelFilePath: "../kinematics/ur5e.json"},
	}
	test.That(t, fakeArm.reconfigure(ctx, nil, confFromFile), test.ShouldBeNil)
	models, err = fakeArm.Get3DModels(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(models), test.ShouldEqual, 0)
}
