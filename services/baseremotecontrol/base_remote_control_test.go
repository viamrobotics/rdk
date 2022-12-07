package baseremotecontrol_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/baseremotecontrol"
	rutils "go.viam.com/rdk/utils"
)

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(baseremotecontrol.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	actualSvc := returnMock("svc1")
	reconfSvc, err := baseremotecontrol.WrapWithReconfigurable(actualSvc, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = baseremotecontrol.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("baseremotecontrol.Service", nil))

	reconfSvc2, err := baseremotecontrol.WrapWithReconfigurable(reconfSvc, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc)
}

func TestReconfigure(t *testing.T) {
	actualSvc := returnMock("svc1")
	reconfSvc, err := baseremotecontrol.WrapWithReconfigurable(actualSvc, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc, test.ShouldNotBeNil)

	actualSvc2 := returnMock("svc1")
	reconfSvc2, err := baseremotecontrol.WrapWithReconfigurable(actualSvc2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldNotBeNil)
	test.That(t, actualSvc.reconfCount, test.ShouldEqual, 0)

	err = reconfSvc.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc, test.ShouldResemble, reconfSvc2)
	test.That(t, actualSvc.reconfCount, test.ShouldEqual, 1)

	err = reconfSvc.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfSvc, nil))
}

func returnMock(name string) *mock {
	return &mock{
		name: name,
	}
}

type mock struct {
	baseremotecontrol.Service
	name        string
	reconfCount int
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}
