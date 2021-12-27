package input

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
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
				UUID: "48d8bd5e-629b-51c1-8bf8-f2f308942012",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"input1",
			resource.Name{
				UUID: "bd8e8873-6bf0-52c7-9034-6527a245a943",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
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

	err = fakeInput1.(*reconfigurableInputController).Reconfigure(context.Background(), fakeInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeInput1.(*reconfigurableInputController).actual, test.ShouldEqual, actualInput2)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)

	err = fakeInput1.(*reconfigurableInputController).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new Controller")
}

type mock struct {
	Controller
	Name        string
	reconfCount int
}

func (m *mock) Close() { m.reconfCount++ }
