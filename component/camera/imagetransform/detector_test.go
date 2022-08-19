package imagetransform

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/vision"
)

func writeTempConfig(cfg *config.Config) (string, error) {
	newConf, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return "", err
	}
	tmpFile, err := ioutil.TempFile(os.TempDir(), "objdet_config-")
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
	cfg, err := config.Read(context.Background(), "data/vision.json", logger)
	if err != nil {
		return nil, err
	}
	// create fake source camera
	cameraComp := config.Component{
		Name:  "fake_cam",
		Type:  camera.SubtypeName,
		Model: "file",
		Attributes: config.AttributeMap{
			"color":   artifact.MustPath("vision/objectdetection/detection_test.jpg"),
			"depth":   "",
			"aligned": false,
		},
	}
	cfg.Components = append(cfg.Components, cameraComp)
	// create fake detector camera
	detectorComp := config.Component{
		Name:  "color_detect",
		Type:  camera.SubtypeName,
		Model: "detector",
		Attributes: config.AttributeMap{
			"source":        "fake_cam",
			"detector_name": "detector_color",
		},
		DependsOn: []string{"fake_cam"},
	}
	cfg.Components = append(cfg.Components, detectorComp)
	tfliteComp := config.Component{
		Name:  "tflite_detect",
		Type:  camera.SubtypeName,
		Model: "detector",
		Attributes: config.AttributeMap{
			"source":               "fake_cam",
			"detector_name":        "detector_tflite",
			"confidence_threshold": 0.35,
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

	// add the detector
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	detConf := vision.DetectorConfig{
		Name: "detector_color",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#4F3815",
			"tolerance":    0.013,
			"segment_size": 15000,
		},
	}
	err = srv.AddDetector(context.Background(), detConf)
	test.That(t, err, test.ShouldBeNil)

	detector, err := camera.FromRobot(r, "color_detect")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := detector.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(852, 431), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(984, 561), test.ShouldResemble, rimage.Red)
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
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	detConf := vision.DetectorConfig{
		Name: "detector_tflite",
		Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  artifact.MustPath("vision/tflite/effdet0.tflite"),
			"num_threads": 1,
		},
	}
	err = srv.AddDetector(context.Background(), detConf)
	test.That(t, err, test.ShouldBeNil)

	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := detector.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	test.That(t, ovImg.GetXY(624, 402), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(816, 648), test.ShouldResemble, rimage.Red)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	logger := golog.NewDevelopmentLogger("benchmark-color")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	// add the detector
	srv, err := vision.FromRobot(r)
	test.That(b, err, test.ShouldBeNil)
	detConf := vision.DetectorConfig{
		Name: "detector_color",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#4F3815",
			"tolerance":    0.055556,
			"segment_size": 15000,
		},
	}
	err = srv.AddDetector(context.Background(), detConf)
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "color_detect")
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(ctx)
	}
}

func BenchmarkTFLiteDetectionSource(b *testing.B) {
	logger := golog.NewDevelopmentLogger("benchmark-tflite")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := buildRobotWithFakeCamera(logger)
	defer func() {
		test.That(b, r.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(b, err, test.ShouldBeNil)
	// add the detector
	srv, err := vision.FromRobot(r)
	test.That(b, err, test.ShouldBeNil)
	detConf := vision.DetectorConfig{
		Name: "detector_tflite",
		Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  artifact.MustPath("vision/tflite/effdet0.tflite"),
			"num_threads": 1,
		},
	}
	err = srv.AddDetector(context.Background(), detConf)
	test.That(b, err, test.ShouldBeNil)
	detector, err := camera.FromRobot(r, "tflite_detect")
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(ctx)
	}
}
