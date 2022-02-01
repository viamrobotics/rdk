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
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case sensor.Named(testSensorName):
			return sensor1, true
		case sensor.Named(testSensorName2):
			return "not a sensor", false
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{sensor.Named(testSensorName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, ok := sensor.FromRobot(r, testSensorName)
	test.That(t, s, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	result, err := s.GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})

	s, ok = sensor.FromRobot(r, testSensorName2)
	test.That(t, s, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	s, ok = sensor.FromRobot(r, fakeSensorName)
	test.That(t, s, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
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
				UUID: "0434a3a1-3bf4-5f98-8ca7-3bee0487f970",
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
				UUID: "abfe61a0-61ed-523e-9793-f0d5dded2915",
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
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

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
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 0)

	err = reconfSensor1.Reconfigure(context.Background(), reconfSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor1, test.ShouldResemble, reconfSensor2)
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 1)

	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 0)
	test.That(t, actualSensor2.readingsCalls, test.ShouldEqual, 0)
	result, err := reconfSensor1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})
	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 0)
	test.That(t, actualSensor2.readingsCalls, test.ShouldEqual, 1)

	err = reconfSensor1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new Sensor")
}

func TestGetReadings(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, _ := sensor.WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 0)
	result, err := reconfSensor1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})
	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualSensor1 := &mock{Name: testSensorName}
	reconfSensor1, _ := sensor.WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfSensor1), test.ShouldBeNil)
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 1)
}

var reading = 1.5

type mock struct {
	sensor.Sensor
	Name          string
	readingsCalls int
	reconfCalls   int
}

func (m *mock) GetReadings(ctx context.Context) ([]interface{}, error) {
	m.readingsCalls++
	return []interface{}{reading}, nil
}

func (m *mock) Close() { m.reconfCalls++ }
