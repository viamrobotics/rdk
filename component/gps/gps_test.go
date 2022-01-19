package gps

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
)

func TestGPSName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "047fe0db-e1e8-5b26-b7a6-6e5814eaf4b3",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"gps1",
			resource.Name{
				UUID: "07c9cc8d-f36d-5f7d-a114-5a38b96a148c",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "gps1",
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
	var actualGPS1 GPS = &mock{Name: "gps1"}
	reconfGPS1, err := WrapWithReconfigurable(actualGPS1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGPS1.(*reconfigurableGPS).actual, test.ShouldEqual, actualGPS1)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	fakeGPS2, err := WrapWithReconfigurable(reconfGPS1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeGPS2, test.ShouldEqual, reconfGPS1)
}

func TestReconfigurableGPS(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, err := WrapWithReconfigurable(actualGPS1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGPS1.(*reconfigurableGPS).actual, test.ShouldEqual, actualGPS1)

	actualGPS2 := &mock{Name: "gps2"}
	fakeGPS2, err := WrapWithReconfigurable(actualGPS2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGPS1.reconfCalls, test.ShouldEqual, 0)

	err = reconfGPS1.(*reconfigurableGPS).Reconfigure(context.Background(), fakeGPS2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGPS1.(*reconfigurableGPS).actual, test.ShouldEqual, actualGPS2)
	test.That(t, actualGPS1.reconfCalls, test.ShouldEqual, 1)

	err = reconfGPS1.(*reconfigurableGPS).Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new GPS")
}

func TestLocation(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.locCalls, test.ShouldEqual, 0)
	loc1, err := reconfGPS1.(*reconfigurableGPS).Location(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(90, 1))
	test.That(t, actualGPS1.locCalls, test.ShouldEqual, 1)
}

func TestAltitude(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.altCalls, test.ShouldEqual, 0)
	alt1, err := reconfGPS1.(*reconfigurableGPS).Altitude(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alt1, test.ShouldAlmostEqual, alt)
	test.That(t, actualGPS1.altCalls, test.ShouldEqual, 1)
}

func TestSpeed(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.speedCalls, test.ShouldEqual, 0)
	speed1, err := reconfGPS1.(*reconfigurableGPS).Speed(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1, test.ShouldAlmostEqual, speed)
	test.That(t, actualGPS1.speedCalls, test.ShouldEqual, 1)
}

func TestSatellites(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.satCalls, test.ShouldEqual, 0)
	actualSats1, totalSats1, err := reconfGPS1.(*reconfigurableGPS).Satellites(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualSats1, test.ShouldEqual, activeSats)
	test.That(t, totalSats1, test.ShouldEqual, totalSats)
	test.That(t, actualGPS1.satCalls, test.ShouldEqual, 1)
}

func TestAccuracy(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.accCalls, test.ShouldEqual, 0)
	hAcc1, vAcc1, err := reconfGPS1.(*reconfigurableGPS).Accuracy(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hAcc1, test.ShouldAlmostEqual, hAcc)
	test.That(t, vAcc1, test.ShouldAlmostEqual, vAcc)
	test.That(t, actualGPS1.accCalls, test.ShouldEqual, 1)
}

func TestValid(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.validCalls, test.ShouldEqual, 0)
	valid1, err := reconfGPS1.(*reconfigurableGPS).Valid(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, valid1, test.ShouldEqual, valid)
	test.That(t, actualGPS1.validCalls, test.ShouldEqual, 1)
}

func TestReadings(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.readingsCalls, test.ShouldEqual, 0)
	result, err := reconfGPS1.(*reconfigurableGPS).Readings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{loc})
	test.That(t, actualGPS1.readingsCalls, test.ShouldEqual, 1)
}

func TestDesc(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.descCalls, test.ShouldEqual, 0)
	result := reconfGPS1.(*reconfigurableGPS).Desc()
	test.That(t, result, test.ShouldResemble, desc)
	test.That(t, actualGPS1.descCalls, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualGPS1 := &mock{Name: "gps1"}
	reconfGPS1, _ := WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.reconfCalls, test.ShouldEqual, 0)
	test.That(t, reconfGPS1.(*reconfigurableGPS).Close(context.Background()), test.ShouldBeNil)
	test.That(t, actualGPS1.reconfCalls, test.ShouldEqual, 1)
}

var (
	loc        = geo.NewPoint(90, 1)
	alt        = 50.5
	speed      = 5.4
	activeSats = 1
	totalSats  = 2
	hAcc       = 0.7
	vAcc       = 0.8
	valid      = true
	desc       = sensor.Description{sensor.Type("gps"), ""}
)

type mock struct {
	GPS
	Name          string
	locCalls      int
	altCalls      int
	speedCalls    int
	satCalls      int
	accCalls      int
	validCalls    int
	readingsCalls int
	descCalls     int
	reconfCalls   int
}

// Location always returns the set values.
func (m *mock) Location(ctx context.Context) (*geo.Point, error) {
	m.locCalls++
	return loc, nil
}

// Altitude returns the set value.
func (m *mock) Altitude(ctx context.Context) (float64, error) {
	m.altCalls++
	return alt, nil
}

// Speed returns the set value.
func (m *mock) Speed(ctx context.Context) (float64, error) {
	m.speedCalls++
	return speed, nil
}

// Satellites returns the set values.
func (m *mock) Satellites(ctx context.Context) (int, int, error) {
	m.satCalls++
	return activeSats, totalSats, nil
}

// Accuracy returns the set values.
func (m *mock) Accuracy(ctx context.Context) (float64, float64, error) {
	m.accCalls++
	return hAcc, vAcc, nil
}

// Valid returns the set value.
func (m *mock) Valid(ctx context.Context) (bool, error) {
	m.validCalls++
	return valid, nil
}

func (m *mock) Readings(ctx context.Context) ([]interface{}, error) {
	m.readingsCalls++
	return []interface{}{loc}, nil
}

func (m *mock) Desc(ctx context.Context) (sensor.Description, error) {
	m.descCalls++
	return desc, nil
}
func (m *mock) Close() { m.reconfCalls++ }
