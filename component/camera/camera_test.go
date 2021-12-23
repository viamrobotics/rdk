package camera

import (
	"testing"

	"go.viam.com/rdk/resource"

	"go.viam.com/test"
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
				UUID: "cf359663-1ffd-553f-8d2a-bd99656aa19a",
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
				UUID: "6543c4be-b3dd-56ab-976a-48cd37c91501",
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

	err = fakeCamera1.(*reconfigurableCamera).Reconfigure(fakeCamera2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCamera1.(*reconfigurableCamera).actual, test.ShouldEqual, actualCamera2)
	test.That(t, actualCamera1.reconfCount, test.ShouldEqual, 1)

	err = fakeCamera1.(*reconfigurableCamera).Reconfigure(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new camera")
}

type mock struct {
	Camera
	Name        string
	reconfCount int
}

func (m *mock) Close() error { m.reconfCount++; return nil }
