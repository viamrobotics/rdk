package transformpipeline

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/viamrobotics/gostream"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

type streamTest struct{}

// Next will stream a color image.
func (*streamTest) Next(ctx context.Context) (image.Image, func(), error) {
	return rimage.NewImage(1280, 720), func() {}, nil
}
func (*streamTest) Close(ctx context.Context) error { return nil }

func TestComposed(t *testing.T) {
	// create pointcloud source and fake robot
	robot := &inject.Robot{}
	logger := logging.NewTestLogger(t)
	cloudSource := &inject.Camera{}
	cloudSource.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		p := pointcloud.New()
		return p, p.Set(pointcloud.NewVector(0, 0, 0), pointcloud.NewColoredData(color.NRGBA{255, 1, 2, 255}))
	}
	cloudSource.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return &streamTest{}, nil
	}
	cloudSource.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	// get intrinsic parameters, and make config
	am := utils.AttributeMap{
		"intrinsic_parameters": &transform.PinholeCameraIntrinsics{
			Width:  1280,
			Height: 720,
			Fx:     100,
			Fy:     100,
		},
	}
	// make transform pipeline, expected result with correct config
	conf := &transformConfig{
		Pipeline: []Transformation{
			{
				Type:       "overlay",
				Attributes: am,
			},
		},
	}
	pc, err := cloudSource.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 1)

	myOverlay, stream, err := newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)
	pic, _, err := camera.ReadImage(context.Background(), myOverlay)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1280, 720))

	myPipeline, err := newTransformPipeline(context.Background(), cloudSource, conf, robot, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myPipeline.Close(context.Background())
	pic, _, err = camera.ReadImage(context.Background(), myPipeline)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1280, 720))

	// wrong result with bad config
	_, _, err = newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, utils.AttributeMap{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	// wrong config, still no intrinsics
	am = utils.AttributeMap{
		"intrinsic_parameters": &transform.PinholeCameraIntrinsics{
			Width:  1280,
			Height: 720,
		},
	}
	_, _, err = newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, am)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	// wrong config, no attributes
	conf = &transformConfig{
		Pipeline: []Transformation{
			{
				Type: "overlay", // no attributes
			},
		},
	}
	_, err = newTransformPipeline(context.Background(), cloudSource, conf, robot, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
}
