package fake

import (
	"context"
	"strings"
	"testing"

	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"

	robotimpl "go.viam.com/rdk/robot/impl"
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
	model, err = referenceframe.ParseModelJSONFile(conf2.ConvertedAttributes.(*Config).ModelFilePath, cfg.Name)
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

	err = fakeArm.Reconfigure(context.Background(), nil, conf3)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "fake arm built with zero degrees-of-freedom")
	test.That(t, fakeArm.joints.Values, test.ShouldResemble, modelJoints)
	test.That(t, fakeArm.model, test.ShouldResemble, model)
}

func configFromJSON(tb testing.TB, jsonData string) *config.Config {
	tb.Helper()
	logger := logging.NewTestLogger(tb)
	conf, err := config.FromReader(context.Background(), "", strings.NewReader(jsonData), logger)
	test.That(tb, err, test.ShouldBeNil)
	return conf
}

func TestFromRobot(t *testing.T) {
	jsonData := `{
		"components": [
			{
				"name": "arm1",
				"type": "arm",
				"model": "fake",
				"attributes": {
					"model-path": "fake_model.json"
				}
			}
		]
	}`

	conf := configFromJSON(t, jsonData)
	logger := logging.NewTestLogger(t)
	r := robotimpl.SetupLocalRobot(t, context.Background(), conf, logger)

	expected := []string{"arm1"}
	testutils.VerifySameElements(t, arm.NamesFromRobot(r), expected)

	_, err := arm.FromRobot(r, "arm1")
	test.That(t, err, test.ShouldBeNil)

	_, err = arm.FromRobot(r, "arm0")
	test.That(t, err, test.ShouldNotBeNil)
}
