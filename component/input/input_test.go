package input

import (
	"testing"

	"go.viam.com/rdk/resource"

	"go.viam.com/test"
)

func TestInputControllerName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "6c851c39-bb5d-5f94-b09d-8b84043c327d",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"input1",
			resource.Name{
				UUID: "954e620a-f0e5-5f24-b065-2847b26fe006",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "input1",
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
	var actualInput1 Controller = &mock{Name: "input1"}
	fakeInput1, err := WrapWithReconfigurable(actualInput1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeInput1.(*reconfigurableInputController).actual, test.ShouldEqual, actualInput1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeInput2, err := WrapWithReconfigurable(fakeInput1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeInput2, test.ShouldEqual, fakeInput1)
}

func TestReconfigurableInputController(t *testing.T) {
	actualInput1 := &mock{Name: "input1"}
	fakeInput1, err := WrapWithReconfigurable(actualInput1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeInput1.(*reconfigurableInputController).actual, test.ShouldEqual, actualInput1)

	actualInput2 := &mock{Name: "input2"}
	fakeInput2, err := WrapWithReconfigurable(actualInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 0)

	err = fakeInput1.(*reconfigurableInputController).Reconfigure(fakeInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeInput1.(*reconfigurableInputController).actual, test.ShouldEqual, actualInput2)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)

	err = fakeInput1.(*reconfigurableInputController).Reconfigure(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new Controller")
}

type mock struct {
	Controller
	Name        string
	reconfCount int
}

func (m *mock) Close() error { m.reconfCount++; return nil }
