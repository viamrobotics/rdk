package sensors_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
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
		svc, err := sensors.FromRobot(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)

		test.That(t, svc1.sensorsCount, test.ShouldEqual, 0)
		result, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, names)
		test.That(t, svc1.sensorsCount, test.ShouldEqual, 1)
	})

	t.Run("not sensors service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "not sensor", nil
		}

		svc, err := sensors.FromRobot(r)
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("sensors.Service", "string"))
		test.That(t, svc, test.ShouldBeNil)
	})

	t.Run("no sensors service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, rutils.NewResourceNotFoundError(name)
		}

		svc, err := sensors.FromRobot(r)
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(sensors.Name))
		test.That(t, svc, test.ShouldBeNil)
	})
}

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{imu.Named("imu")}
	}

	t.Run("resource not found", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, rutils.NewResourceNotFoundError(name)
		}
		_, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(imu.Named("imu")))
	})

	t.Run("no error", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "something", nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)
	})
}

func TestGetSensors(t *testing.T) {
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	sensorNames := []resource.Name{imu.Named("imu"), gps.Named("gps")}
	r.ResourceNamesFunc = func() []resource.Name {
		return sensorNames
	}

	t.Run("no sensors", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "something", nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		names, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldBeEmpty)
	})

	t.Run("one sensor", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			if name == imu.Named("imu") {
				return &inject.Sensor{}, nil
			}
			return "something", nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(imu.Named("imu")))
	})

	t.Run("many sensors", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return &inject.Sensor{}, nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}

func TestGetReadings(t *testing.T) {
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	sensorNames := []resource.Name{imu.Named("imu"), gps.Named("gps"), gps.Named("gps2")}
	r.ResourceNamesFunc = func() []resource.Name {
		return sensorNames
	}

	t.Run("no sensors", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "something", nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu")})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a registered sensor")
	})

	t.Run("failing sensor", func(t *testing.T) {
		injectSensor := &inject.Sensor{}
		passedErr := errors.New("can't get readings")
		injectSensor.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return nil, passedErr
		}
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return injectSensor, nil
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu")})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(passedErr, "failed to get reading from %q", imu.Named("imu")))
	})

	t.Run("many sensors", func(t *testing.T) {
		readings1 := []interface{}{1.1, 2.2}
		injectSensor := &inject.Sensor{}
		injectSensor.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return readings1, nil
		}
		readings2 := []interface{}{2.2, 3.3}
		injectSensor2 := &inject.Sensor{}
		injectSensor2.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return readings2, nil
		}
		injectSensor3 := &inject.Sensor{}
		passedErr := errors.New("can't read")
		injectSensor3.GetReadingsFunc = func(ctx context.Context) ([]interface{}, error) {
			return nil, passedErr
		}
		expected := map[resource.Name]interface{}{
			imu.Named("imu"): readings1,
			gps.Named("gps"): readings2,
		}
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			switch name {
			case imu.Named("imu"):
				return injectSensor, nil
			case gps.Named("gps"):
				return injectSensor2, nil
			case gps.Named("gps2"):
				return injectSensor3, nil
			}
			return nil, rutils.NewResourceNotFoundError(name)
		}
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu2")})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a registered sensor")

		readings, err := svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 1)
		reading := readings[0]
		test.That(t, reading.Name, test.ShouldResemble, imu.Named("imu"))
		test.That(t, reading.Readings, test.ShouldResemble, readings1)

		readings, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu"), imu.Named("imu"), imu.Named("imu")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 1)
		reading = readings[0]
		test.That(t, reading.Name, test.ShouldResemble, imu.Named("imu"))
		test.That(t, reading.Readings, test.ShouldResemble, readings1)

		readings, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu"), gps.Named("gps")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 2)
		test.That(t, readings[0].Readings, test.ShouldResemble, expected[readings[0].Name])
		test.That(t, readings[1].Readings, test.ShouldResemble, expected[readings[1].Name])

		_, err = svc.GetReadings(context.Background(), sensorNames)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(passedErr, "failed to get reading from %q", gps.Named("gps2")))
	})
}

func TestUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	r := &inject.Robot{}
	sensorNames := []resource.Name{imu.Named("imu"), gps.Named("gps")}
	r.ResourceNamesFunc = func() []resource.Name {
		return sensorNames
	}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return &inject.Sensor{}, nil
	}

	t.Run("update with no sensors", func(t *testing.T) {
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{imu.Named("imu"): "not sensor"})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sNames1, test.ShouldBeEmpty)
	})

	t.Run("update with one sensor", func(t *testing.T) {
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(imu.Named("imu")))
	})

	t.Run("update with same sensors", func(t *testing.T) {
		svc, err := sensors.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.(resource.Updateable).Update(
			context.Background(),
			map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}, gps.Named("gps"): &inject.Sensor{}},
		)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}

var names = []resource.Name{gps.Named("gps"), imu.Named("imu")}

type mock struct {
	sensors.Service

	sensorsCount int
}

func (m *mock) GetSensors(ctx context.Context) ([]resource.Name, error) {
	m.sensorsCount++
	return names, nil
}
