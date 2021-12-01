package gripper

import (
	"testing"

	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func TestGripperName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "c0ee7310-504c-5e57-8386-dfd75372c242",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"gripper1",
			resource.Name{
				UUID: "169be933-0c65-58bc-be9c-2a9fbe6c70c9",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "gripper1",
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
	var actualGripper1 Gripper = &mock{Name: "gripper1"}
	fakeGripper1, err := WrapWithReconfigurable(actualGripper1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGripper1.(*reconfigurableGripper).actual, test.ShouldEqual, actualGripper1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeGripper2, err := WrapWithReconfigurable(fakeGripper1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGripper2, test.ShouldEqual, fakeGripper1)
}

func TestReconfigurableGripper(t *testing.T) {
	actualGripper1 := &mock{Name: "gripper1"}
	fakeGripper1, err := WrapWithReconfigurable(actualGripper1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGripper1.(*reconfigurableGripper).actual, test.ShouldEqual, actualGripper1)

	actualGripper2 := &mock{Name: "gripper2"}
	fakeGripper2, err := WrapWithReconfigurable(actualGripper2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGripper1.reconCount, test.ShouldEqual, 0)

	err = fakeGripper1.(*reconfigurableGripper).Reconfigure(fakeGripper2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGripper1.(*reconfigurableGripper).actual, test.ShouldEqual, actualGripper2)
	test.That(t, actualGripper1.reconCount, test.ShouldEqual, 1)

	err = fakeGripper1.(*reconfigurableGripper).Reconfigure(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new gripper")
}

type mock struct {
	Gripper
	Name       string
	reconCount int
}

func (m *mock) Close() error { m.reconCount++; return nil }
