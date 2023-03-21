package vision

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot/packages"
)

func TestAttributesWalker(t *testing.T) {
	makeVisionAttributes := func(modelPath, labelPath string) *Attributes {
		return &Attributes{
			ModelRegistry: []VisModelConfig{
				{
					Name: "my_classifier",
					Type: "classifications",
					Parameters: config.AttributeMap{
						"model_path":  modelPath,
						"label_path":  labelPath,
						"num_threads": 1,
					},
				},
			},
		}
	}

	visionAttrs := makeVisionAttributes("/some/path/on/robot/model.tflite", "/other/path/on/robot/textFile.txt")
	visionAttrsWithRefs := makeVisionAttributes("${packages.test_model}/model.tflite", "${packages.test_model}/textFile.txt")
	visionAttrsOneRef := makeVisionAttributes("/some/path/on/robot/model.tflite", "${packages.test_model}/textFile.txt")

	packageManager := packages.NewNoopManager()

	testAttributesWalker := func(t *testing.T, attrs *Attributes, expectedModelPath, expectedLabelPath string) {
		newAttrs, err := attrs.Walk(packages.NewPackagePathVisitor(packageManager))
		test.That(t, err, test.ShouldBeNil)

		test.That(t, newAttrs.(*Attributes).ModelRegistry, test.ShouldNotBeNil)
		test.That(t, newAttrs.(*Attributes).ModelRegistry, test.ShouldHaveLength, 1)
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Name, test.ShouldEqual, "my_classifier")
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Type, test.ShouldEqual, "classifications")
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters, test.ShouldNotBeNil)
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["model_path"], test.ShouldEqual, expectedModelPath)
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["label_path"], test.ShouldEqual, expectedLabelPath)
		test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["num_threads"], test.ShouldEqual, 1)
	}

	testAttributesWalker(t, visionAttrs, "/some/path/on/robot/model.tflite", "/other/path/on/robot/textFile.txt")
	testAttributesWalker(t, visionAttrsWithRefs, "test_model/model.tflite", "test_model/textFile.txt")
	testAttributesWalker(t, visionAttrsOneRef, "/some/path/on/robot/model.tflite", "test_model/textFile.txt")
}
