package objectdetection_test

import (
	"context"
	"encoding/json"
	"image"
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
	"go.viam.com/rdk/services/objectdetection"
	"go.viam.com/rdk/services/objectsegmentation"
)

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
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	// add the detector
	detConf := objectdetection.Config{
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

func TestDetectorNames(t *testing.T) {
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.RobotFromConfigPath(context.Background(), "data/fake.json", logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	names, err := srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "detector_3")
	// test if detector was added to segmentation service
	segSrv, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	segNames, err := segSrv.GetSegmenters(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segNames, test.ShouldContain, "detector_3")
}

func TestDetect(t *testing.T) {
	r := buildRobotWithFakeCamera(t)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	dets, err := srv.Detect(context.Background(), "fake_cam", "detect_red")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dets, test.ShouldHaveLength, 1)
	test.That(t, dets[0].Label(), test.ShouldEqual, "red")
	test.That(t, dets[0].Score(), test.ShouldEqual, 1.0)
	box := dets[0].BoundingBox()
	test.That(t, box.Min, test.ShouldResemble, image.Point{110, 288})
	test.That(t, box.Max, test.ShouldResemble, image.Point{183, 349})
	// errors
	_, err = srv.Detect(context.Background(), "fake_cam", "detect_blue")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	_, err = srv.Detect(context.Background(), "real_cam", "detect_red")
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"rdk:component:camera/real_cam\" not found")
}

func TestAddDetector(t *testing.T) {
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.RobotFromConfigPath(context.Background(), "data/empty.json", logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := objectdetection.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	// success
	cfg := objectdetection.Config{
		Name: "test",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	ok, err := srv.AddDetector(context.Background(), cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	names, err := srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	// test if detector was added to segmentation service
	segSrv, err := objectsegmentation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	segNames, err := segSrv.GetSegmenters(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segNames, test.ShouldContain, "test")
	// failure
	cfg.Name = "will_fail"
	cfg.Type = "wrong_type"
	ok, err = srv.AddDetector(context.Background(), cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "is not implemented")
	test.That(t, ok, test.ShouldBeFalse)
	names, err = srv.DetectorNames(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, names, test.ShouldContain, "test")
	test.That(t, names, test.ShouldNotContain, "will_fail")
	segNames, err = segSrv.GetSegmenters(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, segNames, test.ShouldNotContain, "will_fail")
}
