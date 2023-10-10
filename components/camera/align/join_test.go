package align

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestJoinWithImages(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cam := inject.NewCamera("cam")
	params := &transform.PinholeCameraIntrinsics{ // D435 intrinsics for 424x240
		Width:  424,
		Height: 240,
		Fx:     304.1299133300781,
		Fy:     304.2772216796875,
		Ppx:    213.47967529296875,
		Ppy:    124.63351440429688,
	}
	img, err := rimage.NewImageFromFile(artifact.MustPath("pointcloud/the_color_image_intel_424.jpg"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("pointcloud/the_depth_image_intel_424.png"))
	test.That(t, err, test.ShouldBeNil)
	cam.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{
			IntrinsicParams: params,
		}, nil
	}
	cam.ImagesFunc = func(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		imgs := []camera.NamedImage{{img, "color"}, {dm, "depth"}}
		return imgs, resource.ResponseMetadata{}, nil
	}
	cfg := &joinConfig{
		ImageType: "color",
		Color:     "intel",
		Depth:     "intel",
	}
	joinCam, err := newJoinColorDepth(context.Background(), cam, cam, resource.Name{Name: "foo"}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	alignedPointCloud, err := joinCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alignedPointCloud, test.ShouldNotBeNil)
	tempPCD, err := os.CreateTemp(t.TempDir(), "*.pcd")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tempPCD.Name())
	err = pointcloud.ToPCD(alignedPointCloud, tempPCD, pointcloud.PCDBinary)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, joinCam.Close(context.Background()), test.ShouldBeNil)
}

func TestJoin(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.Read(context.Background(), utils.ResolveFile("components/camera/align/data/join_cam.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	c := conf.FindComponent("join_cam")
	test.That(t, c, test.ShouldNotBeNil)

	joinConf, ok := c.ConvertedAttributes.(*joinConfig)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, joinConf, test.ShouldNotBeNil)
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board2.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)
	// create the source cameras
	colorSrc := &videosource.StaticSource{ColorImg: img}
	colorVideoSrc, err := camera.NewVideoSourceFromReader(context.Background(), colorSrc, nil, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	depthSrc := &videosource.StaticSource{DepthImg: dm}
	depthVideoSrc, err := camera.NewVideoSourceFromReader(context.Background(), depthSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)

	// create the join camera
	is, err := newJoinColorDepth(context.Background(), colorVideoSrc, depthVideoSrc, resource.Name{Name: "foo"}, joinConf, logger)
	test.That(t, err, test.ShouldBeNil)
	// get images and point clouds
	alignedPointCloud, err := is.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alignedPointCloud, test.ShouldNotBeNil)
	outImage, _, err := camera.ReadImage(context.Background(), is)
	test.That(t, err, test.ShouldBeNil)
	outDepth, ok := outImage.(*rimage.DepthMap)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, outDepth, test.ShouldNotBeNil)

	test.That(t, colorVideoSrc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, depthVideoSrc.Close(context.Background()), test.ShouldBeNil)
	test.That(t, is.Close(context.Background()), test.ShouldBeNil)
	// set necessary fields to nil, expect errors
	joinConf.CameraParameters = nil
	_, err = newJoinColorDepth(context.Background(), colorVideoSrc, depthVideoSrc, resource.Name{Name: "foo"}, joinConf, logger)
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
}
