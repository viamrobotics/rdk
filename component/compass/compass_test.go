package compass

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/sensor"
)

func TestCompassName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "3c4145b6-aff8-52b9-9b06-778abc940d0f",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"compass1",
			resource.Name{
				UUID: "286ec871-7aa7-5eba-98c0-6c3da28cdccb",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "compass1",
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
	var actualCompass1 Compass = &mock{}
	reconfCompass1, err := WrapWithReconfigurable(actualCompass1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfCompass1.(*reconfigurableCompass).actual, test.ShouldEqual, actualCompass1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeCompass2, err := WrapWithReconfigurable(reconfCompass1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeCompass2, test.ShouldEqual, reconfCompass1)
}

func TestReconfigurableCompass(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, err := WrapWithReconfigurable(actualCompass1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfCompass1.(*reconfigurableCompass).actual, test.ShouldEqual, actualCompass1)

	actualCompass2 := &mock{}
	fakeCompass2, err := WrapWithReconfigurable(actualCompass2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCompass1.reconfCalls, test.ShouldEqual, 0)

	err = reconfCompass1.(*reconfigurableCompass).Reconfigure(context.Background(), fakeCompass2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfCompass1.(*reconfigurableCompass).actual, test.ShouldEqual, actualCompass2)
	test.That(t, actualCompass1.reconfCalls, test.ShouldEqual, 1)

	err = reconfCompass1.(*reconfigurableCompass).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new Compass")
}

func TestHeading(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.headingCalls, test.ShouldEqual, 0)
	heading1, err := reconfCompass1.(Compass).Heading(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading1, test.ShouldAlmostEqual, 1.5)
	test.That(t, actualCompass1.headingCalls, test.ShouldEqual, 1)
}

func TestStartCalibration(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.startCalCalls, test.ShouldEqual, 0)
	err := reconfCompass1.(Compass).StartCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCompass1.startCalCalls, test.ShouldEqual, 1)
}

func TestStopCalibration(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.stopCalCalls, test.ShouldEqual, 0)
	err := reconfCompass1.(Compass).StopCalibration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCompass1.stopCalCalls, test.ShouldEqual, 1)
}

func TestMark(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	err := reconfCompass1.(*reconfigurableCompass).Mark(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not Markable")

	actualCompass2 := &mockMarkable{}
	reconfCompass2, _ := WrapWithReconfigurable(actualCompass2)

	test.That(t, actualCompass2.markCalls, test.ShouldEqual, 0)
	err = reconfCompass2.(*reconfigurableCompass).Mark(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualCompass2.markCalls, test.ShouldEqual, 1)
}

func TestReadings(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.readingsCalls, test.ShouldEqual, 0)
	result, err := reconfCompass1.(*reconfigurableCompass).Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{heading})
	test.That(t, actualCompass1.readingsCalls, test.ShouldEqual, 1)
}

func TestDesc(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.descCalls, test.ShouldEqual, 0)
	result := reconfCompass1.(*reconfigurableCompass).Desc()
	test.That(t, result, test.ShouldResemble, desc)
	test.That(t, actualCompass1.descCalls, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualCompass1 := &mock{}
	reconfCompass1, _ := WrapWithReconfigurable(actualCompass1)

	test.That(t, actualCompass1.reconfCalls, test.ShouldEqual, 0)
	test.That(t, reconfCompass1.(*reconfigurableCompass).Close(context.Background()), test.ShouldBeNil)
	test.That(t, actualCompass1.reconfCalls, test.ShouldEqual, 1)
}

var (
	heading = 1.5
	desc    = sensor.Description{sensor.Type("compass"), ""}
)

type mock struct {
	Compass
	headingCalls  int
	startCalCalls int
	stopCalCalls  int
	readingsCalls int
	descCalls     int
	reconfCalls   int

	HeadingFunc func(ctx context.Context) (float64, error)
}

func (m *mock) Heading(ctx context.Context) (float64, error) {
	m.headingCalls++
	if m.HeadingFunc == nil {
		return heading, nil
	}
	return m.HeadingFunc(ctx)
}

func (m *mock) StartCalibration(ctx context.Context) error {
	m.startCalCalls++
	return nil
}

func (m *mock) StopCalibration(ctx context.Context) error {
	m.stopCalCalls++
	return nil
}

func (m *mock) Readings(ctx context.Context) ([]interface{}, error) {
	m.readingsCalls++
	return []interface{}{heading}, nil
}

func (m *mock) Desc() sensor.Description {
	m.descCalls++
	return desc
}
func (m *mock) Close() { m.reconfCalls++ }

type mockMarkable struct {
	mock
	markCalls int
}

func (m *mockMarkable) Mark(ctx context.Context) error {
	m.markCalls++
	return nil
}

func TestMedianHeading(t *testing.T) {
	dev := &mock{}
	err1 := errors.New("whoops")
	dev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 0, err1
	}
	_, err := MedianHeading(context.Background(), dev)
	test.That(t, err, test.ShouldEqual, err1)

	readings := []float64{1, 2, 3, 4, 4, 2, 4, 4, 1, 1, 2}
	readCount := 0
	dev.HeadingFunc = func(ctx context.Context) (float64, error) {
		reading := readings[readCount]
		readCount++
		return reading, nil
	}
	med, err := MedianHeading(context.Background(), dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, med, test.ShouldEqual, 3)
}
