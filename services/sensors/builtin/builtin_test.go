package builtin_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors/builtin"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestNew(t *testing.T) {
	logger := logging.NewTestLogger(t)
	t.Run("no error", func(t *testing.T) {
		deps := make(resource.Dependencies)
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)
	})
}

func TestGetSensors(t *testing.T) {
	logger := logging.NewTestLogger(t)
	sensorNames := []resource.Name{movementsensor.Named("imu"), movementsensor.Named("gps")}
	deps := make(resource.Dependencies)

	t.Run("no sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"): testutils.NewUnimplementedResource(movementsensor.Named("resource")),
			movementsensor.Named("gps"): testutils.NewUnimplementedResource(movementsensor.Named("resource")),
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		names, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, names, test.ShouldBeEmpty)
	})

	t.Run("one sensor", func(t *testing.T) {
		resourceMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"): &inject.Sensor{},
			movementsensor.Named("gps"): testutils.NewUnimplementedResource(movementsensor.Named("resource")),
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(
			t,
			testutils.NewResourceNameSet(sNames1...),
			test.ShouldResemble,
			testutils.NewResourceNameSet(movementsensor.Named("imu")),
		)
	})

	t.Run("many sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"): &inject.Sensor{},
			movementsensor.Named("gps"): &inject.Sensor{},
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}

func TestReadings(t *testing.T) {
	logger := logging.NewTestLogger(t)
	sensorNames := []resource.Name{movementsensor.Named("imu"), movementsensor.Named("gps"), movementsensor.Named("gps2")}
	deps := make(resource.Dependencies)

	t.Run("no sensors", func(t *testing.T) {
		resourceMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"):  testutils.NewUnimplementedResource(movementsensor.Named("resource")),
			movementsensor.Named("gps"):  testutils.NewUnimplementedResource(movementsensor.Named("resource")),
			movementsensor.Named("gps2"): testutils.NewUnimplementedResource(movementsensor.Named("resource")),
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Readings(context.Background(), []resource.Name{movementsensor.Named("imu")}, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a registered sensor")
	})

	t.Run("failing sensor", func(t *testing.T) {
		injectSensor := &inject.Sensor{}
		passedErr := errors.New("can't get readings")
		injectSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return nil, passedErr
		}
		failMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"):  injectSensor,
			movementsensor.Named("gps"):  injectSensor,
			movementsensor.Named("gps2"): injectSensor,
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), failMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Readings(context.Background(), []resource.Name{movementsensor.Named("imu")}, map[string]interface{}{})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(passedErr, "failed to get reading from %q", movementsensor.Named("imu")))
	})

	t.Run("many sensors", func(t *testing.T) {
		readings1 := map[string]interface{}{"a": 1.1, "b": 2.2}
		injectSensor := &inject.Sensor{}
		injectSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return readings1, nil
		}
		readings2 := map[string]interface{}{"a": 2.2, "b": 3.3}
		injectSensor2 := &inject.Sensor{}
		injectSensor2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return readings2, nil
		}
		injectSensor3 := &inject.Sensor{}
		passedErr := errors.New("can't read")
		injectSensor3.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
			return nil, passedErr
		}
		expected := map[resource.Name]interface{}{
			movementsensor.Named("imu"): readings1,
			movementsensor.Named("gps"): readings2,
		}
		resourceMap := map[resource.Name]resource.Resource{
			movementsensor.Named("imu"): injectSensor,
			movementsensor.Named("gps"): injectSensor2, movementsensor.Named("gps2"): injectSensor3,
		}
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Readings(context.Background(), []resource.Name{movementsensor.Named("imu2")}, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, "not a registered sensor")

		readings, err := svc.Readings(context.Background(), []resource.Name{movementsensor.Named("imu")}, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 1)
		reading := readings[0]
		test.That(t, reading.Name, test.ShouldResemble, movementsensor.Named("imu"))
		test.That(t, reading.Readings, test.ShouldResemble, readings1)

		readings, err = svc.Readings(
			context.Background(),
			[]resource.Name{movementsensor.Named("imu"), movementsensor.Named("imu"), movementsensor.Named("imu")},
			map[string]interface{}{},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 1)
		reading = readings[0]
		test.That(t, reading.Name, test.ShouldResemble, movementsensor.Named("imu"))
		test.That(t, reading.Readings, test.ShouldResemble, readings1)

		readings, err = svc.Readings(
			context.Background(),
			[]resource.Name{movementsensor.Named("imu"), movementsensor.Named("gps")},
			map[string]interface{}{},
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 2)
		test.That(t, readings[0].Readings, test.ShouldResemble, expected[readings[0].Name])
		test.That(t, readings[1].Readings, test.ShouldResemble, expected[readings[1].Name])

		_, err = svc.Readings(context.Background(), sensorNames, map[string]interface{}{})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(passedErr, "failed to get reading from %q", movementsensor.Named("gps2")))
	})
}

func TestReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)
	sensorNames := []resource.Name{movementsensor.Named("imu"), movementsensor.Named("gps")}
	resourceMap := map[resource.Name]resource.Resource{
		movementsensor.Named("imu"): &inject.Sensor{},
		movementsensor.Named("gps"): &inject.Sensor{},
	}
	deps := make(resource.Dependencies)

	t.Run("update with no sensors", func(t *testing.T) {
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.Reconfigure(
			context.Background(),
			map[resource.Name]resource.Resource{
				movementsensor.Named("imu"): testutils.NewUnimplementedResource(generic.Named("not sensor")),
			},
			resource.Config{},
		)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sNames1, test.ShouldBeEmpty)
	})

	t.Run("update with one sensor", func(t *testing.T) {
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.Reconfigure(
			context.Background(),
			map[resource.Name]resource.Resource{movementsensor.Named("imu"): &inject.Sensor{}},
			resource.Config{},
		)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(
			t,
			testutils.NewResourceNameSet(sNames1...),
			test.ShouldResemble,
			testutils.NewResourceNameSet(movementsensor.Named("imu")),
		)
	})

	t.Run("update with same sensors", func(t *testing.T) {
		svc, err := builtin.NewBuiltIn(context.Background(), deps, resource.Config{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.Reconfigure(context.Background(), resourceMap, resource.Config{})
		test.That(t, err, test.ShouldBeNil)

		sNames1, err := svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))

		err = svc.Reconfigure(
			context.Background(),
			map[resource.Name]resource.Resource{movementsensor.Named("imu"): &inject.Sensor{}, movementsensor.Named("gps"): &inject.Sensor{}},
			resource.Config{},
		)
		test.That(t, err, test.ShouldBeNil)

		sNames1, err = svc.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, testutils.NewResourceNameSet(sNames1...), test.ShouldResemble, testutils.NewResourceNameSet(sensorNames...))
	})
}
