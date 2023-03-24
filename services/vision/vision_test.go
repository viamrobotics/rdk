package vision

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/utils"
)

func TestConfigWalker(t *testing.T) {
	makeVisionConfig := func(modelPath, labelPath string) *Config {
		return &Config{
			ModelRegistry: []VisModelConfig{
				{
					Name: "my_classifier",
					Type: "classifications",
					Parameters: utils.AttributeMap{
						"model_path":  modelPath,
						"label_path":  labelPath,
						"num_threads": 1,
					},
				},
			},
		}
	}

	visionConf := makeVisionConfig("/some/path/on/robot/model.tflite", "/other/path/on/robot/textFile.txt")
	visionConfWithRefs := makeVisionConfig("${packages.test_model}/model.tflite", "${packages.test_model}/textFile.txt")
	visionConfOneRef := makeVisionConfig("/some/path/on/robot/model.tflite", "${packages.test_model}/textFile.txt")

	packageManager := packages.NewNoopManager()

	testConfigWalker := func(t *testing.T, conf *Config, expectedModelPath, expectedLabelPath string) {
		newConf, err := conf.Walk(packages.NewPackagePathVisitor(packageManager))
		test.That(t, err, test.ShouldBeNil)

		test.That(t, newConf.(*Config).ModelRegistry, test.ShouldNotBeNil)
		test.That(t, newConf.(*Config).ModelRegistry, test.ShouldHaveLength, 1)
		test.That(t, newConf.(*Config).ModelRegistry[0].Name, test.ShouldEqual, "my_classifier")
		test.That(t, newConf.(*Config).ModelRegistry[0].Type, test.ShouldEqual, "classifications")
		test.That(t, newConf.(*Config).ModelRegistry[0].Parameters, test.ShouldNotBeNil)
		test.That(t, newConf.(*Config).ModelRegistry[0].Parameters["model_path"], test.ShouldEqual, expectedModelPath)
		test.That(t, newConf.(*Config).ModelRegistry[0].Parameters["label_path"], test.ShouldEqual, expectedLabelPath)
		test.That(t, newConf.(*Config).ModelRegistry[0].Parameters["num_threads"], test.ShouldEqual, 1)
	}

	testConfigWalker(t, visionConf, "/some/path/on/robot/model.tflite", "/other/path/on/robot/textFile.txt")
	testConfigWalker(t, visionConfWithRefs, "test_model/model.tflite", "test_model/textFile.txt")
	testConfigWalker(t, visionConfOneRef, "/some/path/on/robot/model.tflite", "test_model/textFile.txt")
}
