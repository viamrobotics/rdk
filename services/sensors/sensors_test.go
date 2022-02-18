package sensors_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return svc1, true
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := sensors.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	test.That(t, svc1.sensorsCount, test.ShouldEqual, 0)
	result, err := svc.GetSensors(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, names)
	test.That(t, svc1.sensorsCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return "not sensor", true
	}

	svc, err = sensors.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("sensors.Service", "string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return nil, false
	}

	svc, err = sensors.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(sensors.Name))
	test.That(t, svc, test.ShouldBeNil)
}

// test new

// test get sensors

// test get readings

// test update

var (
	names = []resource.Name{gps.Named("gps"), imu.Named("imu")}
)

type mock struct {
	sensors.Service

	sensorsCount  int
	readingsCount int
}

func (m *mock) GetSensors(ctx context.Context) ([]resource.Name, error) {
	m.sensorsCount++
	return names, nil
}
