package transformpipeline

import (
	"context"
	"encoding/json"
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

func writeTempConfig(cfg *config.Config) (string, error) {
	newConf, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return "", err
	}
	tmpFile, err := os.CreateTemp(os.TempDir(), "objdet_config-")
	if err != nil {
		return "", err
	}
	_, err = tmpFile.Write(newConf)
	if err != nil {
		return "", err
	}
	err = tmpFile.Close()
	if err != nil {
		return "", err
	}
	return tmpFile.Name(), nil
}

// make a fake robot with a vision service.
func buildRobotWithFakeCamera(logger golog.Logger) (robot.Robot, error) {
	// add a fake camera to the config
	cfg, err := config.Read(context.Background(), artifact.MustPath("components/camera/transformpipeline/vision.json"), logger)
	if err != nil {
		return nil, err
	}
	// create fake source camera
	colorSrv1 := config.Service{
		Name:  "detector_color",
		Type:  vision.SubtypeName,
		Model: resource.NewDefaultModel("color_detector"),
		Attributes: config.AttributeMap{
			"detect_color":      "#4F3815",
			"hue_tolerance_pct": 0.013,
			"segment_size_px":   15000,
		},
	}
	cfg.Services = append(cfg.Services, colorSrv1)
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
		Model: resource.NewDefaultModel("ml_model"),
		Attributes: config.AttributeMap{
			"ml_model_name": "object_classifier",
		},
		DependsOn: []string{"object_classifier"},
	}
	cfg.Services = append(cfg.Services, visionSrv1)
	tfliteSrv2 := config.Service{
		Name:  "detector_tflite",
		Type:  mlmodel.SubtypeName,
		Model: resource.NewDefaultModel("tflite_cpu"),
		Attributes: config.AttributeMap{
			"model_path":  artifact.MustPath("vision/tflite/effdet0.tflite"),
			"label_path":  artifact.MustPath("vision/tflite/effdetlabels.txt"),
			"num_threads": 1,
		},
	}
	cfg.Services = append(cfg.Services, tfliteSrv2)
	visionSrv2 := config.Service{
		Name:  "vision_detector",
		Type:  vision.SubtypeName,
		Model: resource.NewDefaultModel("ml_model"),
		Attributes: config.AttributeMap{
			"ml_model_name": "detector_tflite",
		},
		DependsOn: []string{"detector_tflite"},
	}
	cfg.Services = append(cfg.Services, visionSrv2)
	cameraComp := config.Component{
		Name:  "fake_cam",
		Type:  camera.SubtypeName,
		Model: resource.NewDefaultModel("image_file"),
		Attributes: config.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/objectdetection/detection_test.jpg"),
			"depth_image_file_path": "",
		},
	}
	cfg.Components = append(cfg.Components, cameraComp)
	// create fake detector camera
	detectorComp := config.Component{
		Name:  "color_detect",
		Type:  camera.SubtypeName,
		Model: resource.NewDefaultModel("transform"),
		Attributes: config.AttributeMap{
			"source": "fake_cam",
			"pipeline": []config.AttributeMap{
				{
					"type": "detections",
					"attributes": config.AttributeMap{
						"detector_name":        "detector_color",
						"confidence_threshold": 0.35,
					},
				},
			},
		},
		DependsOn: []string{"fake_cam"},
	}
	cfg.Components = append(cfg.Components, detectorComp)
	// create 2nd fake detector camera
	tfliteComp := config.Component{
		Name:  "tflite_detect",
		Type:  camera.SubtypeName,
		Model: resource.NewDefaultModel("transform"),
		Attributes: config.AttributeMap{
			"source": "fake_cam",
			"pipeline": []config.AttributeMap{
				{
					"type": "detections",
					"attributes": config.AttributeMap{
						"detector_name":        "vision_detector",
						"confidence_threshold": 0.35,
					},
				},
			},
		},
		DependsOn: []string{"fake_cam"},
	}
	cfg.Components = append(cfg.Components, tfliteComp)
	newConfFile, err := writeTempConfig(cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(newConfFile)
	// make the robot from new config
	return robotimpl.RobotFromConfigPath(context.Background(), newConfFile, logger)
}

func TestColorDetectionSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()

	detector, err := camera.FromRobot(r, "color_detect")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := camera.ReadImage(ctx, detector)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(852, 431), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(984, 561), test.ShouldResemble, rimage.Red)
	test.That(t, detector.Close(context.Background()), test.ShouldBeNil)
}

func TestTFLiteDetectionSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(t, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := camera.ReadImage(ctx, detector)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(624, 458), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(716, 627), test.ShouldResemble, rimage.Red)
	test.That(t, detector.Close(context.Background()), test.ShouldBeNil)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	logger := golog.NewDebugLogger("benchmark-color")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "color_detect")
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = camera.ReadImage(ctx, detector)
	}
	test.That(b, detector.Close(context.Background()), test.ShouldBeNil)
}

func BenchmarkTFLiteDetectionSource(b *testing.B) {
	logger := golog.NewDebugLogger("benchmark-tflite")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = camera.ReadImage(ctx, detector)
	}
	test.That(b, detector.Close(context.Background()), test.ShouldBeNil)
}
