package gantry

import (
	"testing"

	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func TestGantryName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "b51ae74c-d7ab-5af9-a307-6d3b9ad45347",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"gantry1",
			resource.Name{
				UUID: "5cfef262-55ef-5d44-9630-e77677df09be",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "gantry1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
		})
	}
}

func TestWrapWtihReconfigurable(t *testing.T) {
	var actualGantry1 Gantry = &mock{Name: "gantry1"}
	fakeGantry1, err := WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGantry1.(*reconfigurableGantry).actual, test.ShouldEqual, actualGantry1)
}

func TestReconfigurableGantry(t *testing.T) {
	actualGantry1 := &mock{Name: "gantry1"}
	fakeGantry1, err := WrapWithReconfigurable(actualGantry1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGantry1.(*reconfigurableGantry).actual, test.ShouldEqual, actualGantry1)

	actualGantry2 := &mock{Name: "gantry2"}
	fakeGantry2, err := WrapWithReconfigurable(actualGantry2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGantry1.reconCount, test.ShouldEqual, 0)

	err = fakeGantry1.(*reconfigurableGantry).Reconfigure(fakeGantry2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGantry1.(*reconfigurableGantry).actual, test.ShouldEqual, actualGantry2)
	test.That(t, actualGantry1.reconCount, test.ShouldEqual, 1)
}

type mock struct {
	Gantry
	Name       string
	reconCount int
}

func (m *mock) Close() error { m.reconCount++; return nil }
