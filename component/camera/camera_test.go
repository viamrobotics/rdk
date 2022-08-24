package camera_test

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
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
	ret, err := c.Do(context.Background(), command)
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
	test.That(t, err, test.ShouldBeError, rutils.DependencyTypeError(fakeCameraName, "Camera", "string"))
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
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Camera", "string"))
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
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1)
	test.That(t, err, test.ShouldBeNil)

	_, err = camera.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Camera", nil))

	reconfCamera2, err := camera.WrapWithReconfigurable(reconfCamera1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfCamera2, test.ShouldEqual, reconfCamera1)
}

func TestReconfigurableCamera(t *testing.T) {
	actualCamera1 := &mock{Name: testCameraName}
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1)
	test.That(t, err, test.ShouldBeNil)

	actualCamera2 := &mock{Name: testCameraName2}
	reconfCamera2, err := camera.WrapWithReconfigurable(actualCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 0)

	err = reconfCamera1.Reconfigure(context.Background(), reconfCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rutils.UnwrapProxy(reconfCamera1), test.ShouldResemble, rutils.UnwrapProxy(reconfCamera2))
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 0)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 0)
	img1, _, err := camera.ReadImage(context.Background(), reconfCamera1.(camera.Camera))
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)
	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 0)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 1)
	test.That(t, reconfCamera1.(camera.Camera).Close(context.Background()), test.ShouldBeNil)

	err = reconfCamera1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *camera.reconfigurableCamera")
}

func TestClose(t *testing.T) {
	actualCamera1 := &mock{Name: testCameraName}
	reconfCamera1, err := camera.WrapWithReconfigurable(actualCamera1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfCamera1), test.ShouldBeNil)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 1)
}

var img = image.NewNRGBA(image.Rect(0, 0, 4, 4))

type mock struct {
	camera.Camera
	Name        string
	nextCount   int
	reconfCount int
}

type mockStream struct {
	m *mock
}

func (ms *mockStream) Next(ctx context.Context) (image.Image, func(), error) {
	ms.m.nextCount++
	return img, nil, nil
}

func (ms *mockStream) Close(ctx context.Context) error {
	return nil
}

func (m *mock) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return &mockStream{m: m}, nil
}

func (m *mock) Close(_ context.Context) error { m.reconfCount++; return nil }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath(s.filePath + ".dat.gz"))
	return img, func() {}, err
}

func TestNewCamera(t *testing.T) {
	attrs1 := &camera.AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 1280, Height: 720}}
	attrs2 := &camera.AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 100, Height: 100}}
	videoSrc := &simpleSource{"rimage/board1"}

	// no camera
	_, err := camera.NewFromReader(nil, nil)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil reader"))

	// camera with no camera parameters
	cam1, err := camera.NewFromReader(videoSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	proj, err := cam1.Projector(context.Background())
	test.That(t, proj, test.ShouldBeNil)
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// camera with camera parameters
	proj, _ = camera.GetProjector(context.Background(), attrs1, cam1)
	cam2, err := camera.NewFromReader(videoSrc, proj)
	test.That(t, err, test.ShouldBeNil)
	proj2, err := cam2.Projector(context.Background())
	test.That(t, proj2, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// camera with camera parameters inherited  from other camera
	proj, _ = camera.GetProjector(context.Background(), nil, cam2)
	cam3, err := camera.NewFromReader(videoSrc, proj)
	test.That(t, err, test.ShouldBeNil)
	proj3, err := cam3.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj3, test.ShouldResemble, proj2)

	// camera with different camera parameters, will not inherit
	proj, _ = camera.GetProjector(context.Background(), attrs2, cam2)
	cam4, err := camera.NewFromReader(videoSrc, proj)
	test.That(t, err, test.ShouldBeNil)
	proj4, err := cam4.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj4, test.ShouldNotResemble, proj2)

	// cam4 wrapped with reconfigurable
	reconfig, err := camera.WrapWithReconfigurable(cam4)
	test.That(t, err, test.ShouldBeNil)
	fakeCamera := reconfig.(camera.Camera)
	proj, _ = camera.GetProjector(context.Background(), nil, fakeCamera)
	cam5, err := camera.NewFromReader(videoSrc, proj)
	test.That(t, err, test.ShouldBeNil)
	proj5, err := cam5.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj5, test.ShouldResemble, proj4)
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
	noProj, err := camera.NewFromReader(videoSrc, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = noProj.NextPointCloud(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)
	_, err = noProj.Projector(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// make a camera with a NextPointCloudFunction
	videoSrc2 := &cloudSource{videoSrc, generic.Unimplemented{}}
	noProj2, err := camera.NewFromReader(videoSrc2, nil)
	test.That(t, err, test.ShouldBeNil)
	pc, err := noProj2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, _, err := camera.ReadImage(
		camera.WithMIMETypeHint(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG)),
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
	attrs1 := &camera.AttrConfig{
		CameraParameters: &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
			Width:  1280,
			Height: 720,
			Fx:     200,
			Fy:     200,
			Ppx:    100,
			Ppy:    100,
		},
	}
	proj, _ := camera.GetProjector(context.Background(), attrs1, nil)
	cam, err := camera.NewFromReader(videoSrc, proj)
	test.That(t, err, test.ShouldBeNil)
	pc, err := cam.NextPointCloud(context.Background())
	test.That(t, pc.Size(), test.ShouldEqual, 921600)
	test.That(t, err, test.ShouldBeNil)
	proj, err = cam.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, proj, test.ShouldNotBeNil)
	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)

	// camera with a point cloud function
	videoSrc2 := &cloudSource{videoSrc, generic.Unimplemented{}}
	proj, _ = camera.GetProjector(context.Background(), nil, cam)
	cam2, err := camera.NewFromReader(videoSrc2, proj)
	test.That(t, err, test.ShouldBeNil)
	pc, err = cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, _, err := camera.ReadImage(
		camera.WithMIMETypeHint(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG)),
		cam2)
	test.That(t, err, test.ShouldBeNil)

	depthImg := img.(*rimage.DepthMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthImg.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, depthImg.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}
