package transformpipeline

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/builtin"
	rutils "go.viam.com/rdk/utils"
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
	cameraComp := resource.Config{
		Name:  "fake_cam",
		API:   camera.Subtype,
		Model: resource.NewDefaultModel("image_file"),
		Attributes: rutils.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/objectdetection/detection_test.jpg"),
			"depth_image_file_path": "",
		},
	}
	cfg.Components = append(cfg.Components, cameraComp)
	// create fake detector camera
	detectorComp := resource.Config{
		Name:  "color_detect",
		API:   camera.Subtype,
		Model: resource.NewDefaultModel("transform"),
		Attributes: rutils.AttributeMap{
			"source": "fake_cam",
			"pipeline": []rutils.AttributeMap{
				{
					"type": "detections",
					"attributes": rutils.AttributeMap{
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
	tfliteComp := resource.Config{
		Name:  "tflite_detect",
		API:   camera.Subtype,
		Model: resource.NewDefaultModel("transform"),
		Attributes: rutils.AttributeMap{
			"source": "fake_cam",
			"pipeline": []rutils.AttributeMap{
				{
					"type": "detections",
					"attributes": rutils.AttributeMap{
						"detector_name":        "detector_tflite",
						"confidence_threshold": 0.35,
					},
				},
			},
		},
		DependsOn: []string{"fake_cam"},
	}
	cfg.Components = append(cfg.Components, tfliteComp)
	if err := cfg.Ensure(false, logger); err != nil {
		return nil, err
	}

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

	// add the detector

	srv, err := vision.FirstFromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	detConf := vision.VisModelConfig{
		Name: "detector_color",
		Type: "color_detector",
		Parameters: rutils.AttributeMap{
			"detect_color":      "#4F3815",
			"hue_tolerance_pct": 0.013,
			"segment_size_px":   15000,
		},
	}
	err = srv.AddDetector(context.Background(), detConf, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	detector, err := camera.FromRobot(r, "color_detect")
	test.That(t, err, test.ShouldBeNil)
	defer detector.Close(ctx)

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

	// add the detector
	srv, err := vision.FirstFromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	detConf := vision.VisModelConfig{
		Name: "detector_tflite",
		Type: "tflite_detector",
		Parameters: rutils.AttributeMap{
			"model_path":  artifact.MustPath("vision/tflite/effdet0.tflite"),
			"num_threads": 1,
		},
	}
	err = srv.AddDetector(context.Background(), detConf, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(t, err, test.ShouldBeNil)
	defer detector.Close(ctx)

	resImg, _, err := camera.ReadImage(ctx, detector)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(624, 402), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(816, 648), test.ShouldResemble, rimage.Red)
	test.That(t, detector.Close(context.Background()), test.ShouldBeNil)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	logger := golog.NewTestLogger(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	// add the detector
	srv, err := vision.FirstFromRobot(r)
	test.That(b, err, test.ShouldBeNil)
	detConf := vision.VisModelConfig{
		Name: "detector_color",
		Type: "color_detector",
		Parameters: rutils.AttributeMap{
			"detect_color":      "#4F3815",
			"hue_tolerance_pct": 0.055556,
			"segment_size_px":   15000,
		},
	}
	err = srv.AddDetector(context.Background(), detConf, map[string]interface{}{})
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "color_detect")
	test.That(b, err, test.ShouldBeNil)
	defer detector.Close(ctx)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = camera.ReadImage(ctx, detector)
	}
	test.That(b, detector.Close(context.Background()), test.ShouldBeNil)
}

func BenchmarkTFLiteDetectionSource(b *testing.B) {
	logger := golog.NewTestLogger(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	// add the detector
	srv, err := vision.FirstFromRobot(r)
	test.That(b, err, test.ShouldBeNil)
	detConf := vision.VisModelConfig{
		Name: "detector_tflite",
		Type: "tflite_detector",
		Parameters: rutils.AttributeMap{
			"model_path":  artifact.MustPath("vision/tflite/effdet0.tflite"),
			"num_threads": 1,
		},
	}
	err = srv.AddDetector(context.Background(), detConf, map[string]interface{}{})
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(b, err, test.ShouldBeNil)
	defer detector.Close(ctx)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = camera.ReadImage(ctx, detector)
	}
	test.That(b, detector.Close(context.Background()), test.ShouldBeNil)
}
