package vision_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	rutils "go.viam.com/rdk/utils"
)

var (
	testSvcName1 = "svc1"
	testSvcName2 = "svc2"
)

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(vision.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock1{name: testSvcName1}
	reconfSvc1, err := vision.WrapWithReconfigurable(svc, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = vision.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, vision.NewUnimplementedInterfaceError(nil))

	reconfSvc2, err := vision.WrapWithReconfigurable(reconfSvc1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock1{name: testSvcName1}
	reconfSvc1, err := vision.WrapWithReconfigurable(actualSvc1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock1{name: testSvcName2}
	reconfSvc2, err := vision.WrapWithReconfigurable(actualArm2, resource.Name{})
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

func TestDoCommand(t *testing.T) {
	svc := &mock1{name: testSvcName1}

	resp, err := svc.DoCommand(context.Background(), generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, generic.TestCommand)
	test.That(t, svc.cmd, test.ShouldResemble, generic.TestCommand)
}

type mock1 struct {
	vision.Service
	name        string
	reconfCount int
	cmd         map[string]interface{}
}

func (m *mock1) DoCommand(_ context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	m.cmd = cmd
	return cmd, nil
}

func (m *mock1) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}
