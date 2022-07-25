package segmentation_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

func TestColorObjects(t *testing.T) {
	// create camera
	img, err := rimage.ReadBothFromFile(artifact.MustPath("segmentation/aligned_intel/desktop2.both.gz"), true)
	test.That(t, err, test.ShouldBeNil)
	params, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"), "color")
	test.That(t, err, test.ShouldBeNil)
	cameraAttrs := &camera.AttrConfig{CameraParameters: params}
	c := &imagesource.StaticSource{img.Color, img.Depth, params}
	proj, _ := camera.GetProjector(context.Background(), cameraAttrs, nil)
	cam, err := camera.New(c, proj)
	test.That(t, err, test.ShouldBeNil)
	// create config
	cfg := config.AttributeMap{
		"tolerance":             0.025,
		"detect_color":          "#6D2814",
		"mean_k":                50,
		"sigma":                 1.5,
		"min_points_in_segment": 1000,
	}
	// run segmenter
	objects, err := segmentation.ColorObjects(context.Background(), cam, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objects, test.ShouldHaveLength, 1)
	// create config with no mean_k filtering
	cfg = config.AttributeMap{
		"tolerance":             0.025,
		"detect_color":          "#6D2814",
		"mean_k":                -1,
		"sigma":                 1.5,
		"min_points_in_segment": 1000,
	}
	// run segmenter
	objects, err = segmentation.ColorObjects(context.Background(), cam, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, objects, test.ShouldHaveLength, 1)
}

func TestColorObjectsValidate(t *testing.T) {
	cfg := segmentation.ColorObjectsConfig{}
	// tolerance value too big
	cfg.Tolerance = 10.
	err := cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "tolerance must be between 0.0 and 1.0")
	// not a valid color
	cfg.Tolerance = 1.
	cfg.Color = "#GGGGGG"
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "couldn't parse hex")
	// not a valid sigma
	cfg.Color = "#123456"
	cfg.MeanK = 5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "must be greater than 0")
	// not a valid min segment size
	cfg.Sigma = 1
	cfg.MinSegmentSize = -1
	t.Logf("conf: %v", cfg)
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "min_points_in_segment cannot be less than 0")
	// valid
	cfg.MinSegmentSize = 5
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}
