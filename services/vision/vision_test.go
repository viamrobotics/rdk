package vision

import (
	"testing"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/test"
)

func TestAttributesWalker(t *testing.T) {
	visionAttrs := Attributes{
		ModelRegistry: []VisModelConfig{
			{
				Name: "my_classifier",
				Type: "classifications",
				Parameters: config.AttributeMap(map[string]interface{}{
					"model_path":  "${packages.test_model}/model.tflite",
					"label_path":  "${packages.test_model}/textFile.txt",
					"num_threads": 1,
				}),
			},
		},
	}

	expectedAttrs := Attributes{
		ModelRegistry: []VisModelConfig{
			{
				Name: "my_classifier",
				Type: "classifications",
				Parameters: config.AttributeMap(map[string]interface{}{
					"model_path":  "test_model/model.tflite",
					"label_path":  "test_model/textFile.txt",
					"num_threads": 1,
				}),
			},
		},
	}

	// This just returns the package name for the path; hence the expected values
	packageManager := packages.NewNoopManager()
	newAttrs, err := visionAttrs.Walk(packages.NewPackagePathVisitor(packageManager))
	test.That(t, err, test.ShouldBeNil)

}
