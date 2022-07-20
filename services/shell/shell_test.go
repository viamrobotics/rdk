package shell_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/services/shell"
	rutils "go.viam.com/rdk/utils"
)

var (
	testSvcName1 = "svc1"
	testSvcName2 = "svc2"
)

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(shell.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: testSvcName1}
	reconfSvc1, err := shell.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = shell.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("shell.Service", nil))

	reconfSvc2, err := shell.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := shell.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: testSvcName2}
	reconfSvc2, err := shell.WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldNotBeNil)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 0)

	err = reconfSvc1.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldResemble, reconfSvc2)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 1)

	err = reconfSvc1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfSvc1, nil))
}

type mock struct {
	shell.Service
	name        string
	reconfCount int
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}
