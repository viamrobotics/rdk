package transformpipeline

import (
	"context"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/mlmodel/register"
	"go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/register"
	rutils "go.viam.com/rdk/utils"
)

func buildRobotWithClassifier(logger logging.Logger) (robot.Robot, error) {
	cfg := &config.Config{}

	// create fake source camera
	visionSrv1 := resource.Config{
		Name:       "vision_classifier",
		API:        vision.API,
		Model:      resource.DefaultModelFamily.WithModel("fake"),
		Attributes: rutils.AttributeMap{},
	}
	cfg.Services = append(cfg.Services, visionSrv1)
	cameraComp := resource.Config{
		Name:  "fake_cam",
		API:   camera.API,
		Model: resource.DefaultModelFamily.WithModel("image_file"),
		Attributes: rutils.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/classification/keyboard.jpg"),
			"depth_image_file_path": "",
		},
	}
	cfg.Components = append(cfg.Components, cameraComp)

	// create classification transform camera
	classifierComp := resource.Config{
		Name:  "classification_transform_camera",
		API:   camera.API,
		Model: resource.DefaultModelFamily.WithModel("transform"),
		Attributes: rutils.AttributeMap{
			"source": "fake_cam",
			"pipeline": []rutils.AttributeMap{
				{
					"type": "classifications",
					"attributes": rutils.AttributeMap{
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
	if err := cfg.Ensure(false, logger); err != nil {
		return nil, err
	}

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

func TestClassifierSource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithClassifier(logger)
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	classifier, err := camera.FromRobot(r, "classification_transform_camera")
	test.That(t, err, test.ShouldBeNil)
	defer classifier.Close(ctx)

	streamClassifier, ok := classifier.(camera.StreamCamera)
	test.That(t, ok, test.ShouldBeTrue)
	resImg, _, err := camera.ReadImage(ctx, streamClassifier)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)

	// Max classifications was 5, but this image gets classified with just 2 labels, so we
	// test that each label is present.
	test.That(t, ovImg.GetXY(42, 50), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(268, 48), test.ShouldResemble, rimage.Red)
	test.That(t, classifier.Close(context.Background()), test.ShouldBeNil)
}
