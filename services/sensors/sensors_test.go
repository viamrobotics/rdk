package sensors_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils"
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
	t.Run("no error", func(t *testing.T) {
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)
	})
}

func TestGetSensors(t *testing.T) {
	logger := golog.NewTestLogger(t)
	sensorNames := []resource.Name{imu.Named("imu"), movementsensor.Named("gps")}

	t.Run("no sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]interface{}{imu.Named("imu"): "resource", movementsensor.Named("gps"): "resource"}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		names, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldBeEmpty)
	})

	t.Run("one sensor", func(t *testing.T) {
		resourceMap := map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}, movementsensor.Named("gps"): "resource"}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(imu.Named("imu")))
	})

	t.Run("many sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}, movementsensor.Named("gps"): &inject.Sensor{}}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}

func TestGetReadings(t *testing.T) {
	logger := golog.NewTestLogger(t)
	sensorNames := []resource.Name{imu.Named("imu"), movementsensor.Named("gps"), movementsensor.Named("gps2")}

	t.Run("no sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]interface{}{
			imu.Named("imu"): "resource", movementsensor.Named("gps"): "resource", movementsensor.Named("gps2"): "resource",
		}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
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
		failMap := map[resource.Name]interface{}{
			imu.Named("imu"): injectSensor, movementsensor.Named("gps"): injectSensor, movementsensor.Named("gps2"): injectSensor,
		}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), failMap)
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
			imu.Named("imu"):            readings1,
			movementsensor.Named("gps"): readings2,
		}
		resourceMap := map[resource.Name]interface{}{
			imu.Named("imu"): injectSensor, movementsensor.Named("gps"): injectSensor2, movementsensor.Named("gps2"): injectSensor3,
		}
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
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

		readings, err = svc.GetReadings(context.Background(), []resource.Name{imu.Named("imu"), movementsensor.Named("gps")})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 2)
		test.That(t, readings[0].Readings, test.ShouldResemble, expected[readings[0].Name])
		test.That(t, readings[1].Readings, test.ShouldResemble, expected[readings[1].Name])

		_, err = svc.GetReadings(context.Background(), sensorNames)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(passedErr, "failed to get reading from %q", movementsensor.Named("gps2")))
	})
}

func TestUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	sensorNames := []resource.Name{imu.Named("imu"), movementsensor.Named("gps")}
	resourceMap := map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}, movementsensor.Named("gps"): &inject.Sensor{}}

	t.Run("update with no sensors", func(t *testing.T) {
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
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
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
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
		svc, err := sensors.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.(resource.Updateable).Update(
			context.Background(),
			map[resource.Name]interface{}{imu.Named("imu"): &inject.Sensor{}, movementsensor.Named("gps"): &inject.Sensor{}},
		)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.GetSensors(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}

var names = []resource.Name{movementsensor.Named("gps"), imu.Named("imu")}

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(sensors.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: testSvcName1}
	reconfSvc1, err := sensors.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = sensors.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("sensors.Service", nil))

	reconfSvc2, err := sensors.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: testSvcName1}
	reconfSvc1, err := sensors.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: testSvcName2}
	reconfSvc2, err := sensors.WrapWithReconfigurable(actualArm2)
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
	sensors.Service

	sensorsCount int
	name         string
	reconfCount  int
}

func (m *mock) GetSensors(ctx context.Context) ([]resource.Name, error) {
	m.sensorsCount++
	return names, nil
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}
