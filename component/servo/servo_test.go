package servo

import (
	"context"
	// "math"
	"testing"

	// pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func TestServoName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "1921cf8d-7dc4-5f09-bbad-642c61221c48",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"servo1",
			resource.Name{
				UUID: "ecf53524-109e-5649-984d-e93081ebbc30",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "servo1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	actualServo1 := &mockServo{Name: "servo1"}
	fakeServo1, err := WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeServo1.(*reconfigurableServo).actual, test.ShouldEqual, actualServo1)
}

func TestReconfigurableServo(t *testing.T) {
	actualServo1 := &mockServo{Name: "servo1"}
	fakeServo1, err := WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeServo1.(*reconfigurableServo).actual, test.ShouldEqual, actualServo1)

	actualServo2 := &mockServo{Name: "servo2"}
	fakeServo2, err := WrapWithReconfigurable(actualServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 0)

	err = fakeServo1.(*reconfigurableServo).Reconfigure(fakeServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeServo1.(*reconfigurableServo).actual, test.ShouldEqual, actualServo2)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 1)
}

type mockServo struct {
	Name        string
	reconfCount int
}

func (mServo *mockServo) Move(ctx context.Context, angleDegs uint8) error {
	return nil
}

func (mServo *mockServo) AngularOffset(ctx context.Context) (uint8, error) {
	return 0, nil
}

func (mServo *mockServo) Close() error { mServo.reconfCount++; return nil }
