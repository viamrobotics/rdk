package gripper

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
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
				UUID: "e2b52bce-800b-56b7-904c-2f8372ce4623",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"gripper1",
			resource.Name{
				UUID: "f3e34221-62ec-5951-b112-d4cccb47bf61",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
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

func TestWrapWithReconfigurable(t *testing.T) {
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
	test.That(t, actualGripper1.reconfCount, test.ShouldEqual, 0)

	err = fakeGripper1.(*reconfigurableGripper).Reconfigure(context.Background(), fakeGripper2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGripper1.(*reconfigurableGripper).actual, test.ShouldEqual, actualGripper2)
	test.That(t, actualGripper1.reconfCount, test.ShouldEqual, 1)

	err = fakeGripper1.(*reconfigurableGripper).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new gripper")
}

type mock struct {
	Gripper
	Name        string
	reconfCount int
}

func (m *mock) Close() { m.reconfCount++ }
