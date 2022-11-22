package vision_test

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/vision/builtin"
	"go.viam.com/rdk/spatialmath"
)

func createService(t *testing.T) (vision.Service, robot.Robot) {
	t.Helper()
	filePath := "data/empty.json"
	logger := golog.NewTestLogger(t)
	r, err := robotimpl.RobotFromConfigPath(context.Background(), filePath, logger)
	test.That(t, err, test.ShouldBeNil)
	srv, err := vision.FirstFromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	return srv, r
}

func writeTempConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	newConf, err := json.MarshalIndent(cfg, "", " ")
	t.Logf("%v", string(newConf))
	test.That(t, err, test.ShouldBeNil)
	tmpFile, err := os.CreateTemp(os.TempDir(), "objdet_config-")
	test.That(t, err, test.ShouldBeNil)
	_, err = tmpFile.Write(newConf)
	test.That(t, err, test.ShouldBeNil)
	err = tmpFile.Close()
	test.That(t, err, test.ShouldBeNil)
	return tmpFile.Name()
}

func buildRobotWithFakeCamera(t *testing.T) (robot.Robot, error) {
	t.Helper()
	// add a fake camera to the config
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/empty.json", logger)
	if err != nil {
		return nil, err
	}
	cameraComp := config.Component{
		Name:  "fake_cam",
		Type:  camera.SubtypeName,
		Model: "image_file",
		Attributes: config.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/objectdetection/detection_test.jpg"),
			"depth_image_file_path": "",
			"aligned":               false,
		},
	}
	cameraComp2 := config.Component{
		Name:  "fake_cam2",
		Type:  camera.SubtypeName,
		Model: "image_file",
		Attributes: config.AttributeMap{
			"color_image_file_path": artifact.MustPath("vision/tflite/lion.jpeg"),
			"depth_image_file_path": "",
			"aligned":               false,
		},
	}

	if err != nil {
		return nil, err
	}
	cfg.Components = append(cfg.Components, cameraComp)
	cfg.Components = append(cfg.Components, cameraComp2)
	newConfFile := writeTempConfig(t, cfg)
	defer os.Remove(newConfFile)
	// make the robot from new config and get the service
	r, err := robotimpl.RobotFromConfigPath(context.Background(), newConfFile, logger)
	if err != nil {
		return nil, err
	}
	srv, err := vision.FirstFromRobot(r)
	if err != nil {
		return nil, err
	}
	// add the detector
	detConf := vision.VisModelConfig{
		Name: "detect_red",
		Type: string(builtin.ColorDetector),
		Parameters: config.AttributeMap{
			"detect_color":      "#C9131F", // look for red
			"hue_tolerance_pct": 0.05,
			"segment_size_px":   1000,
		},
	}
	err = srv.AddDetector(context.Background(), detConf, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return r, nil
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
	box1, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: -5, Y: -5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2}, "")
	test.That(t, err, test.ShouldBeNil)
	box2, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{X: 5, Y: 5, Z: 5}), r3.Vector{X: 0, Y: 0, Z: 2}, "")
	test.That(t, err, test.ShouldBeNil)
	return []spatialmath.Geometry{box1, box2}
}

type cloudSource struct{}

func (c *cloudSource) Read(ctx context.Context) (image.Image, func(), error) {
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

func (c *cloudSource) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	panic("unimplemented")
}

func (c *cloudSource) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	props, err := c.Properties(ctx)
	if err != nil {
		return nil, err
	}
	proj = props.IntrinsicParams
	return proj, nil
}

func (c *cloudSource) Properties(ctx context.Context) (camera.Properties, error) {
	return camera.Properties{
		SupportsPCD: true,
		IntrinsicParams: &transform.PinholeCameraIntrinsics{
			Width:  1280,
			Height: 720,
			Fx:     200,
			Fy:     200,
			Ppx:    100,
			Ppy:    100,
		},
	}, nil
}

func (c *cloudSource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (c *cloudSource) Close(ctx context.Context) error {
	return nil
}
