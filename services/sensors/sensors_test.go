package sensors_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

var (
	testSvcName1 = "svc1"
	testSvcName2 = "svc2"
)

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	t.Run("found sensors service", func(t *testing.T) {
		svc, err := sensors.FromRobot(r, testSvcName1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)

		test.That(t, svc1.sensorsCount, test.ShouldEqual, 0)
		result, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, names)
		test.That(t, svc1.sensorsCount, test.ShouldEqual, 1)
	})

	t.Run("not sensors service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "not sensor", nil
		}

		svc, err := sensors.FromRobot(r, testSvcName1)
		test.That(t, err, test.ShouldBeError, sensors.NewUnimplementedInterfaceError("string"))
		test.That(t, svc, test.ShouldBeNil)
	})

	t.Run("no sensors service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, rutils.NewResourceNotFoundError(name)
		}

		svc, err := sensors.FromRobot(r, testSvcName1)
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(sensors.Named(testSvcName1)))
		test.That(t, svc, test.ShouldBeNil)
	})
}

var names = []resource.Name{movementsensor.Named("gps"), movementsensor.Named("imu")}

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(sensors.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: testSvcName1}
	reconfSvc1, err := sensors.WrapWithReconfigurable(svc, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = sensors.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, sensors.NewUnimplementedInterfaceError(nil))

	reconfSvc2, err := sensors.WrapWithReconfigurable(reconfSvc1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := sensors.WrapWithReconfigurable(actualSvc1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: testSvcName2}
	reconfSvc2, err := sensors.WrapWithReconfigurable(actualArm2, resource.Name{})
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
	svc := &mock{name: testSvcName1}

	resp, err := svc.DoCommand(context.Background(), generic.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, generic.TestCommand)
	test.That(t, svc.cmd, test.ShouldResemble, generic.TestCommand)
}

type mock struct {
	sensors.Service

	sensorsCount int
	name         string
	reconfCount  int
	cmd          map[string]interface{}
}

func (m *mock) Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
	m.sensorsCount++
	return names, nil
}

func (m *mock) DoCommand(_ context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	m.cmd = cmd
	return cmd, nil
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}
