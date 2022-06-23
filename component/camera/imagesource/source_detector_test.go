package imagesource

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

func writeTempConfig(t testing.TB, cfg *config.Config) string {
	t.Helper()
	newConf, err := json.MarshalIndent(cfg, "", " ")
	t.Logf("%v", string(newConf))
	test.That(t, err, test.ShouldBeNil)
	tmpFile, err := ioutil.TempFile(os.TempDir(), "objdet_config-")
	test.That(t, err, test.ShouldBeNil)
	_, err = tmpFile.Write(newConf)
	test.That(t, err, test.ShouldBeNil)
	err = tmpFile.Close()
	test.That(t, err, test.ShouldBeNil)
	return tmpFile.Name()
}

// make a fake robot with a vision service
func buildRobotWithFakeCamera(t testing.TB) robot.Robot {
	t.Helper()
	// add a fake camera to the config
	logger := golog.NewLogger("detector_test")
	cfg, err := config.Read(context.Background(), "data/vision.json", logger)
	test.That(t, err, test.ShouldBeNil)
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
		Name:  "fake_detect",
		Type:  camera.SubtypeName,
		Model: "detector",
		Attributes: config.AttributeMap{
			"source":        "fake_cam",
			"detector_name": "detector_color",
		},
	}
	cfg.Components = append(cfg.Components, detectorComp)
	tfliteComp := config.Component{
		Name:  "tflite_detect",
		Type:  camera.SubtypeName,
		Model: "detector",
		Attributes: config.AttributeMap{
			"source":        "fake_cam",
			"detector_name": "detector_tflite",
		},
	}
	cfg.Components = append(cfg.Components, tfliteComp)
	newConfFile := writeTempConfig(t, cfg)
	defer os.Remove(newConfFile)
	// make the robot from new config
	r, err := robotimpl.RobotFromConfigPath(context.Background(), newConfFile, logger)
	test.That(t, err, test.ShouldBeNil)
	return r
}

func TestColorDetectionSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := buildRobotWithFakeCamera(t)

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

	detector, err := camera.FromRobot(r, "fake_detect")
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	resImg, _, err := detector.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	ovImg := rimage.ConvertImage(resImg)
	tempFileName := os.TempDir() + "source_detector_color.jpg"
	err = rimage.SaveImage(ovImg, tempFileName)
	t.Logf("image saved at %s", tempFileName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ovImg.GetXY(852, 431), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(985, 563), test.ShouldResemble, rimage.Red)
}

func TestTFLiteDetectionSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := buildRobotWithFakeCamera(t)

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
	tempFileName := os.TempDir() + "source_detector_tflite.jpg"
	err = rimage.SaveImage(ovImg, tempFileName)
	t.Logf("image saved at %s", tempFileName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ovImg.GetXY(852, 431), test.ShouldResemble, rimage.Red)
	test.That(t, ovImg.GetXY(985, 563), test.ShouldResemble, rimage.Red)
}

func BenchmarkColorDetectionSource(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := buildRobotWithFakeCamera(b)
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
	detector, err := camera.FromRobot(r, "fake_detect")
	test.That(b, err, test.ShouldBeNil)
	defer utils.TryClose(ctx, detector)

	b.ResetTimer()
	// begin benchmarking
	for i := 0; i < b.N; i++ {
		_, _, _ = detector.Next(ctx)
	}
}
