package vision

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot/packages"
)

func TestAttributesWalker(t *testing.T) {
	visionAttrs := &Attributes{
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

	packageManager := packages.NewNoopManager()
	newAttrs, err := visionAttrs.Walk(packages.NewPackagePathVisitor(packageManager))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, newAttrs.(*Attributes).ModelRegistry, test.ShouldNotBeNil)
	test.That(t, newAttrs.(*Attributes).ModelRegistry, test.ShouldHaveLength, 1)
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Name, test.ShouldEqual, "my_classifier")
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Type, test.ShouldEqual, "classifications")
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters, test.ShouldNotBeNil)
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["model_path"], test.ShouldEqual, "test_model/model.tflite")
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["label_path"], test.ShouldEqual, "test_model/textFile.txt")
	test.That(t, newAttrs.(*Attributes).ModelRegistry[0].Parameters["num_threads"], test.ShouldEqual, 1)
}
