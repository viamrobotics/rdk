package transformpipeline

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/mlmodel"
	_ "go.viam.com/rdk/services/mlmodel/register"
	"go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/register"
)

func buildRobotWithClassifier(logger golog.Logger) (robot.Robot, error) {
	cfg := &config.Config{}

	// create fake source camera
	tfliteSrv1 := config.Service{
		Name:  "object_classifier",
		Type:  mlmodel.SubtypeName,
		Model: resource.NewDefaultModel("tflite_cpu"),
		Attributes: config.AttributeMap{
			"model_path":  artifact.MustPath("vision/classification/object_classifier.tflite"),
			"label_path":  artifact.MustPath("vision/classification/object_labels.txt"),
			"num_threads": 1,
		},
	}
	cfg.Services = append(cfg.Services, tfliteSrv1)
	visionSrv1 := config.Service{
		Name:  "vision_classifier",
		Type:  vision.SubtypeName,
		Model: resource.NewDefaultModel("mlmodel"),
		Attributes: config.AttributeMap{
			"ml_model_name": "object_classifier",
		},
		DependsOn: []string{"object_classifier"},
	}
	cfg.Services = append(cfg.Services, visionSrv1)
	cameraComp := config.Component{
		Name:  "fake_cam",
		Type:  camera.SubtypeName,
		Model: resource.NewDefaultModel("image_file"),
		Attributes: config.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/classification/keyboard.jpg"),
			"depth_image_file_path": "",
		},
	}
	cfg.Components = append(cfg.Components, cameraComp)

	// create classification transform camera
	classifierComp := config.Component{
		Name:  "classification_transform_camera",
		Type:  camera.SubtypeName,
		Model: resource.NewDefaultModel("transform"),
		Attributes: config.AttributeMap{
			"source": "fake_cam",
			"pipeline": []config.AttributeMap{
				{
					"type": "classifications",
					"attributes": config.AttributeMap{
						"classifier_name":      "vision_classifier",
						"confidence_threshold": 0.35,
						"max_classifications":  5,
					},
				},
			},
		},
		DependsOn: []string{"fake_cam"},
	}
	cfg.Components = append(cfg.Components, classifierComp)

	newConfFile, err := writeTempConfig(cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(newConfFile)

	// make the robot from new config
	r, err := robotimpl.RobotFromConfigPath(context.Background(), newConfFile, logger)
	if err != nil {
		return nil, err
	}
	return r, nil
}

//nolint:dupl
func TestClassifierSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithClassifier(logger)
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	classifier, err := camera.FromRobot(r, "classification_transform_camera")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, classifier)

	resImg, _, err := camera.ReadImage(ctx, classifier)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	// Max classifications was 5, but this image gets classified with just 2 labels, so we
	// test that each label is present.
	test.That(t, ovImg.GetXY(35, 45), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(35, 58), test.ShouldResemble, rimage.Red)
	test.That(t, classifier.Close(context.Background()), test.ShouldBeNil)
}
