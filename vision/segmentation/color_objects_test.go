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
	c := &imagesource.StaticSource{img}
	params, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"), "color")
	test.That(t, err, test.ShouldBeNil)
	cameraAttrs := &camera.AttrConfig{CameraParameters: params}
	cam, err := camera.New(c, cameraAttrs, nil)
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
}
