package camera_test

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testCameraName    = "camera1"
	testCameraName2   = "camera2"
	failCameraName    = "camera3"
	fakeCameraName    = "camera4"
	missingCameraName = "camera5"
)

func setupInjectRobot() *inject.Robot {
	camera1 := &mock{Name: testCameraName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case camera.Named(testCameraName):
			return camera1, true
		case camera.Named(fakeCameraName):
			return "not a camera", false
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named(testCameraName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, ok := camera.FromRobot(r, testCameraName)
	test.That(t, res, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	img1, _, err := res.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)

	res, ok = camera.FromRobot(r, fakeCameraName)
	test.That(t, res, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	res, ok = camera.FromRobot(r, missingCameraName)
	test.That(t, res, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
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
				UUID: "15031593-23e2-5d62-bf05-b9f5286e1794",
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
				UUID: "dcd0244b-6dd0-53e6-a97b-2b427d231302",
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
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

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
	test.That(t, reconfCamera1, test.ShouldResemble, reconfCamera2)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 0)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 0)
	img1, _, err := reconfCamera1.(camera.Camera).Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	compVal, _, err := rimage.CompareImages(img, img1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compVal, test.ShouldEqual, 0)
	test.That(t, actualCamera1.nextCount, test.ShouldEqual, 0)
	test.That(t, actualCamera2.nextCount, test.ShouldEqual, 1)

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

func (m *mock) Next(ctx context.Context) (image.Image, func(), error) {
	m.nextCount++
	return img, nil, nil
}

func (m *mock) Close() { m.reconfCount++ }

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Next(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewImageFromFile(s.filePath)
	return img, func() {}, err
}

func TestNewCamera(t *testing.T) {
	attrs1 := &camera.AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 1280, Height: 720}}
	attrs2 := &camera.AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 100, Height: 100}}
	imgSrc := &simpleSource{artifact.MustPath("rimage/board1.png")}

	_, err := camera.New(nil, nil, nil)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil image source"))

	cam1, err := camera.New(imgSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	_, ok := cam1.(camera.WithProjector)
	test.That(t, ok, test.ShouldBeFalse)

	cam2, err := camera.New(imgSrc, attrs1, cam1)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam2.(camera.WithProjector)
	test.That(t, ok, test.ShouldBeTrue)

	cam3, err := camera.New(imgSrc, nil, cam2)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam3.(camera.WithProjector)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, cam3.(camera.WithProjector).GetProjector(), test.ShouldResemble, cam2.(camera.WithProjector).GetProjector())

	cam4, err := camera.New(imgSrc, attrs2, cam2)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam4.(camera.WithProjector)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, cam4.(camera.WithProjector).GetProjector(), test.ShouldNotResemble, cam2.(camera.WithProjector).GetProjector())

	fakeCamera, err := camera.WrapWithReconfigurable(cam4)
	test.That(t, err, test.ShouldBeNil)
	cam5, err := camera.New(imgSrc, nil, fakeCamera.(camera.Camera))
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam5.(camera.WithProjector)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, cam5.(camera.WithProjector).GetProjector(), test.ShouldResemble, cam4.(camera.WithProjector).GetProjector())
}
