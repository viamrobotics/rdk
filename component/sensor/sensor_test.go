package sensor_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testSensorName    = "sensor1"
	testSensorName2   = "sensor2"
	failSensorName    = "sensor3"
	fakeSensorName    = "sensor4"
	missingSensorName = "sensor5"
)

func setupInjectRobot() *inject.Robot {
	sensor1 := &mock{Name: testSensorName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case sensor.Named(testSensorName):
			return sensor1, nil
		case sensor.Named(fakeSensorName):
			return "not a sensor", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{sensor.Named(testSensorName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	s, err := sensor.FromRobot(r, testSensorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := s.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := sensor.FromRobot(r, testSensorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})

	s, err = sensor.FromRobot(r, fakeSensorName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Sensor", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = sensor.FromRobot(r, missingSensorName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(sensor.Named(missingSensorName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := sensor.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testSensorName})
}

func TestSensorName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: sensor.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testSensorName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: sensor.SubtypeName,
				},
				Name: testSensorName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := sensor.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, err := sensor.WrapWithReconfigurable(actualSensor1)
	test.That(t, err, test.ShouldBeNil)

	_, err = sensor.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Sensor", nil))

	reconfSensor2, err := sensor.WrapWithReconfigurable(reconfSensor1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor2, test.ShouldEqual, reconfSensor1)
}

func TestReconfigurableSensor(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, err := sensor.WrapWithReconfigurable(actualSensor1)
	test.That(t, err, test.ShouldBeNil)

	actualSensor2 := &mock{Name: testSensorName2}
	reconfSensor2, err := sensor.WrapWithReconfigurable(actualSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualSensor1.reconfCount, test.ShouldEqual, 0)

	err = reconfSensor1.Reconfigure(context.Background(), reconfSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor1, test.ShouldResemble, reconfSensor2)
	test.That(t, actualSensor1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualSensor1.readingsCount, test.ShouldEqual, 0)
	test.That(t, actualSensor2.readingsCount, test.ShouldEqual, 0)
	result, err := reconfSensor1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})
	test.That(t, actualSensor1.readingsCount, test.ShouldEqual, 0)
	test.That(t, actualSensor2.readingsCount, test.ShouldEqual, 1)

	err = reconfSensor1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *sensor.reconfigurableSensor")
}

func TestGetReadings(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, _ := sensor.WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.readingsCount, test.ShouldEqual, 0)
	result, err := reconfSensor1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})
	test.That(t, actualSensor1.readingsCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, _ := sensor.WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfSensor1), test.ShouldBeNil)
	test.That(t, actualSensor1.reconfCount, test.ShouldEqual, 1)
}

var reading = 1.5

type mock struct {
	sensor.Sensor
	Name          string
	readingsCount int
	reconfCount   int
}

func (m *mock) GetReadings(ctx context.Context) ([]interface{}, error) {
	m.readingsCount++
	return []interface{}{reading}, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
