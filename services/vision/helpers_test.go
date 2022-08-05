package vision_test

import (
	"context"
	"encoding/json"
	"image"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

func createService(t *testing.T, filePath string) (vision.Service, robot.Robot) {
	t.Helper()
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.RobotFromConfigPath(context.Background(), filePath, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := vision.FromRobot(r, vision.FindVisionName(r))
	test.That(t, err, test.ShouldBeNil)
	return srv, r
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
	srv, err := vision.FromRobot(r, vision.FindVisionName(r))
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
	err = srv.AddDetector(context.Background(), detConf)
	test.That(t, err, test.ShouldBeNil)
	return r
}

var testPointCloud = []r3.Vector{
	pointcloud.NewVector(5, 5, 5),
	pointcloud.NewVector(5, 5, 6),
	pointcloud.NewVector(5, 5, 4),
	pointcloud.NewVector(-5, -5, 5),
	pointcloud.NewVector(-5, -5, 6),
	pointcloud.NewVector(-5, -5, 4),
}

func makeExpectedBoxes(t *testing.T) []spatialmath.Geometry {
	t.Helper()
	box1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: -5, Y: -5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2})
	test.That(t, err, test.ShouldBeNil)
	box2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 5, Y: 5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2})
	test.That(t, err, test.ShouldBeNil)
	return []spatialmath.Geometry{box1, box2}
}

type simpleSource struct{}

func (s *simpleSource) Next(ctx context.Context) (image.Image, func(), error) {
	img := rimage.NewImage(100, 200)
	img.SetXY(20, 10, rimage.Red)
	return img, nil, nil
}

func (s *simpleSource) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type cloudSource struct{}

func (c *cloudSource) Next(ctx context.Context) (image.Image, func(), error) {
	img := rimage.NewImage(100, 200)
	img.SetXY(20, 10, rimage.Red)
	return img, nil, nil
}

func (c *cloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	pcA := pointcloud.New()
	for _, pt := range testPointCloud {
		err := pcA.Set(pt, nil)
		if err != nil {
			return nil, err
		}
	}
	return pcA, nil
}

func (c *cloudSource) GetProperties(ctx context.Context) (rimage.Projector, error) {
	var proj rimage.Projector
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:      1280,
		Height:     720,
		Fx:         200,
		Fy:         200,
		Ppx:        100,
		Ppy:        100,
		Distortion: transform.DistortionModel{},
	}

	proj = intrinsics
	return proj, nil
}

func (c *cloudSource) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
