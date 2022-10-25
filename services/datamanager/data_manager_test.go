package datamanager_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/services/datamanager"
	rutils "go.viam.com/rdk/utils"
)

var (
	testSvcName1 = "svc1"
	testSvcName2 = "svc2"
)

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(datamanager.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: testSvcName1}
	reconfSvc1, err := datamanager.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = datamanager.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, datamanager.NewUnimplementedInterfaceError(nil))

	reconfSvc2, err := datamanager.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := datamanager.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualSvc2 := &mock{name: testSvcName2}
	reconfSvc2, err := datamanager.WrapWithReconfigurable(actualSvc2)
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

func TestExtraOptions(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := datamanager.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualSvc1.extra, test.ShouldEqual, nil)

	reconfSvc1.(datamanager.Service).Sync(context.Background(), map[string]interface{}{"foo": "bar"})
	test.That(t, actualSvc1.extra, test.ShouldResemble, map[string]interface{}{"foo": "bar"})
}

type mock struct {
	datamanager.Service
	name        string
	reconfCount int
	extra       map[string]interface{}
}

func (m *mock) Sync(_ context.Context, extra map[string]interface{}) error {
	m.extra = extra
	return nil
}

func (m *mock) Close(_ context.Context) error {
	m.reconfCount++
	return nil
}
