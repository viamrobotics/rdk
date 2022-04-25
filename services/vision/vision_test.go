package vision_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/vision"
)

func createService(t *testing.T, filePath string) vision.Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.RobotFromConfigPath(context.Background(), filePath, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	return srv
}

func writeTempConfig(t *testing.T, cfg *config.Config) string {
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

func buildRobotWithFakeCamera(t *testing.T) robot.Robot {
	t.Helper()
	// add a fake camera to the config
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/empty.json", logger)
	test.That(t, err, test.ShouldBeNil)
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
	test.That(t, err, test.ShouldBeNil)
	cfg.Components = append(cfg.Components, cameraComp)
	newConfFile := writeTempConfig(t, cfg)
	defer os.Remove(newConfFile)
	// make the robot from new config and get the service
	r, err := robotimpl.RobotFromConfigPath(context.Background(), newConfFile, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := vision.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	// add the detector
	detConf := vision.DetectorConfig{
		Name: "detect_red",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#C9131F", // look for red
			"tolerance":    0.05,
			"segment_size": 1000,
		},
	}
	_, err = srv.AddDetector(context.Background(), detConf)
	test.That(t, err, test.ShouldBeNil)
	return r
}
