package camera_test

import (
	"context"
	"image"
	"sync"
	"testing"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testCameraName    = "camera1"
	depthCameraName   = "camera_depth"
	testCameraName2   = "camera2"
	failCameraName    = "camera3"
	fakeCameraName    = "camera4"
	missingCameraName = "camera5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[camera.Named(testCameraName)] = &mock{Name: testCameraName}
	deps[camera.Named(fakeCameraName)] = "not a camera"
	return deps
}

func setupInjectRobot() *inject.Robot {
	camera1 := &mock{Name: testCameraName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case camera.Named(testCameraName):
			return camera1, nil
		case camera.Named(fakeCameraName):
			return "not a camera", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named(testCameraName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	c, err := camera.FromRobot(r, testCameraName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := c.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := camera.FromDependencies(deps, testCameraName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	img1, _, err := camera.ReadImage(context.Background(), res)
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)
	test.That(t, res.Close(context.Background()), test.ShouldBeNil)

	res, err = camera.FromDependencies(deps, fakeCameraName)
	test.That(t, err, test.ShouldBeError, camera.DependencyTypeError(fakeCameraName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = camera.FromDependencies(deps, missingCameraName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingCameraName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := camera.FromRobot(r, testCameraName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	img1, _, err := camera.ReadImage(context.Background(), res)
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)
	test.That(t, res.Close(context.Background()), test.ShouldBeNil)

	res, err = camera.FromRobot(r, fakeCameraName)
	test.That(t, err, test.ShouldBeError, camera.NewUnimplementedInterfaceError("string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = camera.FromRobot(r, missingCameraName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(camera.Named(missingCameraName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := camera.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testCameraName})
}

func TestCameraName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: camera.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testCameraName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: camera.SubtypeName,
				},
				Name: testCameraName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := camera.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualCamera1 camera.Camera = &mock{Name: testCameraName}
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = camera.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, camera.NewUnimplementedInterfaceError(nil))

	reconfCamera2, err := camera.WrapWithReconfigurable(reconfCamera1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfCamera2, test.ShouldEqual, reconfCamera1)
}

func TestReconfigurableCamera(t *testing.T) {
	actualCamera1 := &mock{Name: testCameraName}
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualCamera2 := &mock{Name: testCameraName2}
	reconfCamera2, err := camera.WrapWithReconfigurable(actualCamera2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	stream, err := reconfCamera1.(camera.Camera).Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	nextImg, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextImg, test.ShouldResemble, img)
	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 1)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 0)

	test.That(t, rutils.UnwrapProxy(reconfCamera1), test.ShouldNotResemble, rutils.UnwrapProxy(reconfCamera2))
	err = reconfCamera1.Reconfigure(context.Background(), reconfCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rutils.UnwrapProxy(reconfCamera1), test.ShouldResemble, rutils.UnwrapProxy(reconfCamera2))

	stream, err = reconfCamera1.(camera.Camera).Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	nextImg, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextImg, test.ShouldResemble, img)
	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 1)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 1)

	img1, _, err := camera.ReadImage(context.Background(), reconfCamera1.(camera.Camera))
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)
	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 1)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 2)
	test.That(t, reconfCamera1.(camera.Camera).Close(context.Background()), test.ShouldBeNil)

	err = reconfCamera1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *camera.reconfigurableCamera")
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
}

func TestClose(t *testing.T) {
	actualCamera1 := &mock{Name: testCameraName}
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	stream, err := reconfCamera1.(camera.Camera).Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	nextImg, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextImg, test.ShouldResemble, img)

	test.That(t, utils.TryClose(context.Background(), reconfCamera1), test.ShouldBeNil)

	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, context.Canceled.Error())
}

var img = image.NewNRGBA(image.Rect(0, 0, 4, 4))

type mock struct {
	camera.Camera
	mu        sync.Mutex
	Name      string
	nextCount int
	closedErr error
}

type mockStream struct {
	m *mock
}

func (ms *mockStream) Next(ctx context.Context) (image.Image, func(), error) {
	ms.m.mu.Lock()
	defer ms.m.mu.Unlock()
	if ms.m.closedErr != nil {
		return nil, nil, ms.m.closedErr
	}
	ms.m.nextCount++
	return img, func() {}, nil
}

func (ms *mockStream) Close(ctx context.Context) error {
	ms.m.mu.Lock()
	defer ms.m.mu.Unlock()
	return nil
}

func (m *mock) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &mockStream{m: m}, nil
}

func (m *mock) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closedErr = context.Canceled
	return nil
}

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath(s.filePath+".dat.gz"))
	return img, func() {}, err
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
	_, err := camera.NewFromReader(context.Background(), nil, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil reader"))

	// camera with no camera parameters
	cam1, err := camera.NewFromReader(context.Background(), videoSrc, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err := cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeFalse)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)
	cam1, err = camera.NewFromReader(context.Background(), videoSrcPCD, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err = cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeTrue)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)

	// camera with camera parameters
	cam2, err := camera.NewFromReader(
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
	cam3, err := camera.NewFromReader(
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
	cam4, err := camera.NewFromReader(
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

	// cam4 wrapped with reconfigurable
	reconfig, err := camera.WrapWithReconfigurable(cam4, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	fakeCamera := reconfig.(camera.Camera)
	props, _ = fakeCamera.Properties(context.Background())
	cam5, err := camera.NewFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: props.IntrinsicParams},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	cam5props, err := cam5.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *(cam5props.IntrinsicParams), test.ShouldResemble, *(cam4props.IntrinsicParams))
}

type cloudSource struct {
	*simpleSource
	generic.Unimplemented
}

func (cs *cloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	p := pointcloud.New()
	return p, p.Set(pointcloud.NewVector(0, 0, 0), nil)
}

func TestCameraWithNoProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	noProj, err := camera.NewFromReader(context.Background(), videoSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	_, err = noProj.NextPointCloud(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
	_, err = noProj.Projector(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// make a camera with a NextPointCloudFunction
	videoSrc2 := &cloudSource{videoSrc, generic.Unimplemented{}}
	noProj2, err := camera.NewFromReader(context.Background(), videoSrc2, nil, camera.DepthStream)
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
	cam, err := camera.NewFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: params1},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, pc.Size(), test.ShouldEqual, 921600)
	test.That(t, err, test.ShouldBeNil)
	proj, err := cam.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj, test.ShouldNotBeNil)
	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)

	// camera with a point cloud function
	videoSrc2 := &cloudSource{videoSrc, generic.Unimplemented{}}
	props, err := cam.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	cam2, err := camera.NewFromReader(
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

	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}
