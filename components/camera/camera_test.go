package camera_test

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/rdk/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rutils "go.viam.com/rdk/utils"
)

const (
	testCameraName    = "camera1"
	depthCameraName   = "camera_depth"
	failCameraName    = "camera2"
	missingCameraName = "camera3"
)

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath(s.filePath+".dat.gz"))
	return img, func() {}, err
}

func (s *simpleSource) Close(ctx context.Context) error {
	return nil
}

type simpleSourceWithPCD struct {
	filePath string
}

func (s *simpleSourceWithPCD) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath(s.filePath+".dat.gz"))
	return img, func() {}, err
}

func (s *simpleSourceWithPCD) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, nil
}

func (s *simpleSourceWithPCD) Close(ctx context.Context) error {
	return nil
}

func TestNewPinholeModelWithBrownConradyDistortion(t *testing.T) {
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  10,
		Height: 10,
		Fx:     1.0,
		Fy:     2.0,
		Ppx:    3.0,
		Ppy:    4.0,
	}
	distortion := &transform.BrownConrady{}

	expected1 := transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics, Distortion: distortion}
	pinholeCameraModel1 := camera.NewPinholeModelWithBrownConradyDistortion(intrinsics, distortion)
	test.That(t, pinholeCameraModel1, test.ShouldResemble, expected1)

	expected2 := transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics}
	pinholeCameraModel2 := camera.NewPinholeModelWithBrownConradyDistortion(intrinsics, nil)
	test.That(t, pinholeCameraModel2, test.ShouldResemble, expected2)
	test.That(t, pinholeCameraModel2.Distortion, test.ShouldBeNil)

	expected3 := transform.PinholeCameraModel{Distortion: distortion}
	pinholeCameraModel3 := camera.NewPinholeModelWithBrownConradyDistortion(nil, distortion)
	test.That(t, pinholeCameraModel3, test.ShouldResemble, expected3)

	expected4 := transform.PinholeCameraModel{}
	pinholeCameraModel4 := camera.NewPinholeModelWithBrownConradyDistortion(nil, nil)
	test.That(t, pinholeCameraModel4, test.ShouldResemble, expected4)
	test.That(t, pinholeCameraModel4.Distortion, test.ShouldBeNil)
}

func TestNewCamera(t *testing.T) {
	intrinsics1 := &transform.PinholeCameraIntrinsics{Width: 128, Height: 72}
	intrinsics2 := &transform.PinholeCameraIntrinsics{Width: 100, Height: 100}
	videoSrc := &simpleSource{"rimage/board1_small"}
	videoSrcPCD := &simpleSourceWithPCD{"rimage/board1_small"}

	// no camera
	_, err := camera.NewVideoSourceFromReader(context.Background(), nil, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil reader"))

	// camera with no camera parameters
	cam1, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err := cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeFalse)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)
	cam1, err = camera.NewVideoSourceFromReader(context.Background(), videoSrcPCD, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err = cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeTrue)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)

	// camera with camera parameters
	cam2, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics1},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	props, err = cam2.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *(props.IntrinsicParams), test.ShouldResemble, *intrinsics1)

	// camera with camera parameters inherited  from other camera
	cam2props, err := cam2.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	cam3, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: cam2props.IntrinsicParams},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	cam3props, err := cam3.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *(cam3props.IntrinsicParams), test.ShouldResemble, *(cam2props.IntrinsicParams))

	// camera with different camera parameters, will not inherit
	cam4, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics2},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	cam4props, err := cam4.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam4props.IntrinsicParams, test.ShouldNotBeNil)
	test.That(t, *(cam4props.IntrinsicParams), test.ShouldNotResemble, *(cam2props.IntrinsicParams))
}

type cloudSource struct {
	resource.Named
	resource.AlwaysRebuild
	*simpleSource
}

func (cs *cloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	p := pointcloud.New()
	return p, p.Set(pointcloud.NewVector(0, 0, 0), nil)
}

func TestCameraWithNoProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	noProj, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	_, err = noProj.NextPointCloud(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
	_, err = noProj.Projector(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// make a camera with a NextPointCloudFunction
	videoSrc2 := &cloudSource{Named: camera.Named("foo").AsNamed(), simpleSource: videoSrc}
	noProj2, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc2, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	pc, err := noProj2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, _, err := camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG)),
		noProj2)
	test.That(t, err, test.ShouldBeNil)

	depthImg := img.(*rimage.DepthMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, depthImg.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, noProj2.Close(context.Background()), test.ShouldBeNil)
}

func TestCameraWithProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	params1 := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	src, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: params1},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	pc, err := src.NextPointCloud(context.Background())
	test.That(t, pc.Size(), test.ShouldEqual, 921600)
	test.That(t, err, test.ShouldBeNil)
	proj, err := src.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj, test.ShouldNotBeNil)
	test.That(t, src.Close(context.Background()), test.ShouldBeNil)

	// camera with a point cloud function
	videoSrc2 := &cloudSource{Named: camera.Named("foo").AsNamed(), simpleSource: videoSrc}
	props, err := src.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	cam2, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc2,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: props.IntrinsicParams},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	pc, err = cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, _, err := camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), rutils.MimeTypePNG),
		cam2)
	test.That(t, err, test.ShouldBeNil)

	depthImg := img.(*rimage.DepthMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, depthImg.Bounds().Dy(), test.ShouldEqual, 720)
	// cam2 should implement a default GetImages, that just returns the one image
	images, _, err := cam2.Images(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(images), test.ShouldEqual, 1)
	test.That(t, images[0].Image, test.ShouldHaveSameTypeAs, &rimage.DepthMap{})
	test.That(t, images[0].Image.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, images[0].Image.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}
