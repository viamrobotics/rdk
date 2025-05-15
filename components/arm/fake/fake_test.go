package fake

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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

func TestXArm6Locations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &Config{
			ModelFilePath: "../example_kinematics/xarm6_kinematics_test.json",
		},
	}

	notReal, err := NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("home location check", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			402.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			102.997474683058328537299531,
			0.000000000000000000000000,
			502.002525316941671462700469,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			131.000000000000000000000000,
			-27.500000000000000000000000,
			274.200000000000000000000000,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			206.000000000000000000000000,
			10.000000000000000000000000,
			141.500000000000000000000000,
		}

		in := make([]referenceframe.Input, 6)
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check1", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-13.477511247321800169629569,
			0.000000000000000000000000,
			401.325562312533520525903441,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			83.174566602088916056345624,
			0.000000000000000000000000,
			516.742582359123957758129109,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			158.566974381217988820935716,
			-27.362614545145717670493468,
			299.589614460540474283334333,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			259.050559200277916716004256,
			18.072894555453149934010071,
			192.543551746158527748775668,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check2", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-26.820359657333263214695762,
			0.000000000000000000000000,
			399.308988008567553151806351,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			60.777555075062835499011271,
			0.000000000000000000000000,
			530.142781900699674224597402,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			180.312201371473605604478507,
			-26.951830890634148829576588,
			333.355009225598280409030849,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			297.258065257027055849903263,
			27.068045067389423508075197,
			256.363984524505951867467957,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
}

func locationCheckTestHelper(geomList []spatialmath.Geometry, checkMap map[string]r3.Vector) bool {
	for _, g := range geomList {
		vecCheck := checkMap[g.Label()]
		vecActual := g.Pose().Point()
		if !spatialmath.R3VectorAlmostEqual(vecCheck, vecActual, 1e-2) {
			return false
		}
	}
	return true
}
