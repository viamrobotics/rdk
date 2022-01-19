package sensor

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

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
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"sensor1",
			resource.Name{
				UUID: "abfe61a0-61ed-523e-9793-f0d5dded2915",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "sensor1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	actualSensor1 := &mock{Name: "sensor1"}
	reconfSensor1, err := WrapWithReconfigurable(actualSensor1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor1.(*reconfigurableSensor).actual, test.ShouldEqual, actualSensor1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeSensor2, err := WrapWithReconfigurable(reconfSensor1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeSensor2, test.ShouldEqual, reconfSensor1)
}

func TestReconfigurableSensor(t *testing.T) {
	actualSensor1 := &mock{Name: "sensor1"}
	reconfSensor1, err := WrapWithReconfigurable(actualSensor1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor1.(*reconfigurableSensor).actual, test.ShouldEqual, actualSensor1)

	actualSensor2 := &mock{Name: "sensor2"}
	fakeSensor2, err := WrapWithReconfigurable(actualSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 0)

	err = reconfSensor1.(*reconfigurableSensor).Reconfigure(context.Background(), fakeSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSensor1.(*reconfigurableSensor).actual, test.ShouldEqual, actualSensor2)
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 1)

	err = reconfSensor1.(*reconfigurableSensor).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new Sensor")
}

func TestReadings(t *testing.T) {
	actualSensor1 := &mock{Name: "sensor1"}
	reconfSensor1, _ := WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 0)
	result, err := reconfSensor1.(*reconfigurableSensor).Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{reading})
	test.That(t, actualSensor1.readingsCalls, test.ShouldEqual, 1)
}

func TestDesc(t *testing.T) {
	actualSensor1 := &mock{Name: "sensor1"}
	reconfSensor1, _ := WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.descCalls, test.ShouldEqual, 0)
	result, err := reconfSensor1.(*reconfigurableSensor).Desc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, desc)
	test.That(t, actualSensor1.descCalls, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualSensor1 := &mock{Name: "sensor1"}
	reconfSensor1, _ := WrapWithReconfigurable(actualSensor1)

	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 0)
	test.That(t, reconfSensor1.(*reconfigurableSensor).Close(context.Background()), test.ShouldBeNil)
	test.That(t, actualSensor1.reconfCalls, test.ShouldEqual, 1)
}

var (
	reading = 1.5
	desc    = Description{Type("sensor"), ""}
)

type mock struct {
	Sensor
	Name          string
	readingsCalls int
	descCalls     int
	reconfCalls   int
}

func (m *mock) Readings(ctx context.Context) ([]interface{}, error) {
	m.readingsCalls++
	return []interface{}{reading}, nil
}

func (m *mock) Desc(context.Context) (Description, error) {
	m.descCalls++
	return desc, nil
}
func (m *mock) Close() { m.reconfCalls++ }
