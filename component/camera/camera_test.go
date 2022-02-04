package camera

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

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
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"camera1",
			resource.Name{
				UUID: "dcd0244b-6dd0-53e6-a97b-2b427d231302",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "camera1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualCamera1 Camera = &mock{Name: "camera1"}
	fakeCamera1, err := WrapWithReconfigurable(actualCamera1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCamera1.(*reconfigurableCamera).actual, test.ShouldEqual, actualCamera1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeCamera2, err := WrapWithReconfigurable(fakeCamera1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCamera2, test.ShouldEqual, fakeCamera1)
}

func TestReconfigurableCamera(t *testing.T) {
	actualCamera1 := &mock{Name: "camera1"}
	fakeCamera1, err := WrapWithReconfigurable(actualCamera1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCamera1.(*reconfigurableCamera).actual, test.ShouldEqual, actualCamera1)

	actualCamera2 := &mock{Name: "camera2"}
	fakeCamera2, err := WrapWithReconfigurable(actualCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 0)

	err = fakeCamera1.(*reconfigurableCamera).Reconfigure(context.Background(), fakeCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCamera1.(*reconfigurableCamera).actual, test.ShouldEqual, actualCamera2)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 1)

	err = fakeCamera1.(*reconfigurableCamera).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new camera")
}

type mock struct {
	Camera
	Name        string
	reconfCount int
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
	attrs1 := &AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 1280, Height: 720}}
	attrs2 := &AttrConfig{CameraParameters: &transform.PinholeCameraIntrinsics{Width: 100, Height: 100}}
	imgSrc := &simpleSource{artifact.MustPath("rimage/board1.png")}

	_, err := New(nil, nil, nil)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil image source"))

	cam1, err := New(imgSrc, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	_, ok := cam1.(WithProjector)
	test.That(t, ok, test.ShouldBeFalse)

	cam2, err := New(imgSrc, attrs1, cam1)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam2.(WithProjector)
	test.That(t, ok, test.ShouldBeTrue)

	cam3, err := New(imgSrc, nil, cam2)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam3.(WithProjector)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, cam3.(WithProjector).GetProjector(), test.ShouldResemble, cam2.(WithProjector).GetProjector())

	cam4, err := New(imgSrc, attrs2, cam2)
	test.That(t, err, test.ShouldBeNil)
	_, ok = cam4.(WithProjector)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, cam4.(WithProjector).GetProjector(), test.ShouldNotResemble, cam2.(WithProjector).GetProjector())
}
