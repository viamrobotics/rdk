package transformpipeline

import (
	"context"
	"image"
	"image/color"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
)

func TestComposed(t *testing.T) {
	// create pointcloud source and fake robot
	robot := &inject.Robot{}
	cloudSource := &inject.Camera{}
	cloudSource.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		p := pointcloud.New()
		return p, p.Set(pointcloud.NewVector(0, 0, 0), pointcloud.NewColoredData(color.NRGBA{255, 1, 2, 255}))
	}
	// get intrinsic parameters, and make config
	am := config.AttributeMap{
		"intrinsic_parameters": &transform.PinholeCameraIntrinsics{
			Width:  1280,
			Height: 720,
			Fx:     100,
			Fy:     100,
		},
	}
	// make transform pipeline, expected result with correct config
	conf := &transformConfig{
		Stream: "color",
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

	myOverlay, err := newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	pic, _, err := camera.ReadImage(context.Background(), myOverlay)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1280, 720))

	myPipeline, err := newTransformPipeline(context.Background(), cloudSource, conf, robot)
	test.That(t, err, test.ShouldBeNil)
	defer myPipeline.Close(context.Background())
	pic, _, err = camera.ReadImage(context.Background(), myPipeline)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1280, 720))

	// wrong result with bad config
	_, err = newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, config.AttributeMap{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	am = config.AttributeMap{
		"intrinsic_parameters": &transform.PinholeCameraIntrinsics{
			Width:  1280,
			Height: 720,
		},
	}
	_, err = newOverlayTransform(context.Background(), cloudSource, camera.ColorStream, am)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
	conf = &transformConfig{
		Stream: "color",
		Pipeline: []Transformation{
			{
				Type: "overlay", // no attributes
			},
		},
	}
	_, err = newTransformPipeline(context.Background(), cloudSource, conf, robot)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldWrap, transform.ErrNoIntrinsics)
}
