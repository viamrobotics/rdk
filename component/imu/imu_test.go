package imu

import (
	"testing"

	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func TestIMUName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "a5fb18ee-d69d-5a8d-b716-c4dac028b93c",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"imu1",
			resource.Name{
				UUID: "23f3b6b6-598f-5659-8b07-7c3dc333efb3",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "imu1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWtihReconfigurable(t *testing.T) {
	var actualIMU1 IMU = &mock{Name: "imu1"}
	fakeIMU1, err := WrapWithReconfigurable(actualIMU1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeIMU1.(*reconfigurableIMU).actual, test.ShouldEqual, actualIMU1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeIMU2, err := WrapWithReconfigurable(fakeIMU1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeIMU2, test.ShouldEqual, fakeIMU1)
}

func TestReconfigurableIMU(t *testing.T) {
	actualIMU1 := &mock{Name: "imu1"}
	fakeIMU1, err := WrapWithReconfigurable(actualIMU1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeIMU1.(*reconfigurableIMU).actual, test.ShouldEqual, actualIMU1)

	actualIMU2 := &mock{Name: "imu2"}
	fakeIMU2, err := WrapWithReconfigurable(actualIMU2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualIMU1.reconfCount, test.ShouldEqual, 0)

	err = fakeIMU1.(*reconfigurableIMU).Reconfigure(fakeIMU2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeIMU1.(*reconfigurableIMU).actual, test.ShouldEqual, actualIMU2)
	test.That(t, actualIMU1.reconfCount, test.ShouldEqual, 1)

	err = fakeIMU1.(*reconfigurableIMU).Reconfigure(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new IMU")
}

type mock struct {
	IMU
	Name        string
	reconfCount int
}

func (m *mock) Close() error { m.reconfCount++; return nil }
