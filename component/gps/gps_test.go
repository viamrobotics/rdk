package gps_test

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testGPSName    = "gps1"
	testGPSName2   = "gps2"
	failGPSName    = "gps3"
	fakeGPSName    = "gps4"
	missingGPSName = "gps5"
)

func setupInjectRobot() *inject.Robot {
	gps1 := &mock{Name: testGPSName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case gps.Named(testGPSName):
			return gps1, nil
		case gps.Named(fakeGPSName):
			return "not a gps", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{gps.Named(testGPSName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	g, err := gps.FromRobot(r, testGPSName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := g.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := gps.FromRobot(r, testGPSName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.ReadLocation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, loc)

	s, err = gps.FromRobot(r, fakeGPSName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("GPS", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = gps.FromRobot(r, missingGPSName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(gps.Named(missingGPSName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := gps.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testGPSName})
}

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
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: gps.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testGPSName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: gps.SubtypeName,
				},
				Name: testGPSName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := gps.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualGPS1 gps.GPS = &mock{Name: testGPSName}
	reconfGPS1, err := gps.WrapWithReconfigurable(actualGPS1)
	test.That(t, err, test.ShouldBeNil)

	_, err = gps.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGPS", nil))

	reconfGPS2, err := gps.WrapWithReconfigurable(reconfGPS1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGPS2, test.ShouldEqual, reconfGPS1)
}

func TestReconfigurableGPS(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, err := gps.WrapWithReconfigurable(actualGPS1)
	test.That(t, err, test.ShouldBeNil)

	actualGPS2 := &mock{Name: testGPSName2}
	reconfGPS2, err := gps.WrapWithReconfigurable(actualGPS2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGPS1.reconfCount, test.ShouldEqual, 0)

	err = reconfGPS1.Reconfigure(context.Background(), reconfGPS2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGPS1, test.ShouldResemble, reconfGPS2)
	test.That(t, actualGPS1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualGPS1.locCount, test.ShouldEqual, 0)
	test.That(t, actualGPS2.locCount, test.ShouldEqual, 0)
	result, err := reconfGPS1.(gps.GPS).ReadLocation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, loc)
	test.That(t, actualGPS1.locCount, test.ShouldEqual, 0)
	test.That(t, actualGPS2.locCount, test.ShouldEqual, 1)

	err = reconfGPS1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *gps.reconfigurableGPS")
}

func TestReadLocation(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.locCount, test.ShouldEqual, 0)
	loc1, err := reconfGPS1.(gps.LocalGPS).ReadLocation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(90, 1))
	test.That(t, actualGPS1.locCount, test.ShouldEqual, 1)
}

func TestReadAltitude(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.altCount, test.ShouldEqual, 0)
	alt1, err := reconfGPS1.(gps.LocalGPS).ReadAltitude(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alt1, test.ShouldAlmostEqual, alt)
	test.That(t, actualGPS1.altCount, test.ShouldEqual, 1)
}

func TestReadSpeed(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.speedCount, test.ShouldEqual, 0)
	speed1, err := reconfGPS1.(gps.LocalGPS).ReadSpeed(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1, test.ShouldAlmostEqual, speed)
	test.That(t, actualGPS1.speedCount, test.ShouldEqual, 1)
}

func TestReadSatellites(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.satCount, test.ShouldEqual, 0)
	actualSats1, totalSats1, err := reconfGPS1.(gps.LocalGPS).ReadSatellites(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualSats1, test.ShouldEqual, activeSats)
	test.That(t, totalSats1, test.ShouldEqual, totalSats)
	test.That(t, actualGPS1.satCount, test.ShouldEqual, 1)
}

func TestReadAccuracy(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.accCount, test.ShouldEqual, 0)
	hAcc1, vAcc1, err := reconfGPS1.(gps.LocalGPS).ReadAccuracy(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, hAcc1, test.ShouldAlmostEqual, hAcc)
	test.That(t, vAcc1, test.ShouldAlmostEqual, vAcc)
	test.That(t, actualGPS1.accCount, test.ShouldEqual, 1)
}

func TestReadValid(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.validCount, test.ShouldEqual, 0)
	valid1, err := reconfGPS1.(gps.LocalGPS).ReadValid(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, valid1, test.ShouldEqual, valid)
	test.That(t, actualGPS1.validCount, test.ShouldEqual, 1)
}

func TestReadFix(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.fixCount, test.ShouldEqual, 0)
	fix1, err := reconfGPS1.(gps.LocalGPS).ReadFix(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
	test.That(t, actualGPS1.fixCount, test.ShouldEqual, 1)
}

func TestGetReadings(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	readings1, err := gps.GetReadings(context.Background(), actualGPS1)
	allReadings := []interface{}{loc.Lat(), loc.Lng(), alt, speed, activeSats, totalSats, hAcc, vAcc, valid, fix}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings1, test.ShouldResemble, allReadings)

	result, err := reconfGPS1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, readings1)

	actualGPS2 := &mockWithSensor{}
	reconfGPS2, _ := gps.WrapWithReconfigurable(actualGPS2)

	test.That(t, actualGPS2.readingsCount, test.ShouldEqual, 0)
	result, err = reconfGPS2.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, readings)
	test.That(t, actualGPS2.readingsCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualGPS1 := &mock{Name: testGPSName}
	reconfGPS1, _ := gps.WrapWithReconfigurable(actualGPS1)

	test.That(t, actualGPS1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfGPS1), test.ShouldBeNil)
	test.That(t, actualGPS1.reconfCount, test.ShouldEqual, 1)
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
	fix        = 1

	readings = []interface{}{5.6, 6.4}
)

type mock struct {
	gps.LocalGPS
	Name        string
	locCount    int
	altCount    int
	speedCount  int
	satCount    int
	accCount    int
	validCount  int
	fixCount    int
	reconfCount int
}

// ReadLocation always returns the set values.
func (m *mock) ReadLocation(ctx context.Context) (*geo.Point, error) {
	m.locCount++
	return loc, nil
}

// ReadAltitude returns the set value.
func (m *mock) ReadAltitude(ctx context.Context) (float64, error) {
	m.altCount++
	return alt, nil
}

// ReadSpeed returns the set value.
func (m *mock) ReadSpeed(ctx context.Context) (float64, error) {
	m.speedCount++
	return speed, nil
}

// ReadSatellites returns the set values.
func (m *mock) ReadSatellites(ctx context.Context) (int, int, error) {
	m.satCount++
	return activeSats, totalSats, nil
}

// ReadAccuracy returns the set values.
func (m *mock) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	m.accCount++
	return hAcc, vAcc, nil
}

// ReadValid returns the set value.
func (m *mock) ReadValid(ctx context.Context) (bool, error) {
	m.validCount++
	return valid, nil
}

// ReadFix returns the set value.
func (m *mock) ReadFix(ctx context.Context) (int, error) {
	m.fixCount++
	return fix, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type mockWithSensor struct {
	mock
	readingsCount int
}

func (m *mockWithSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	m.readingsCount++
	return readings, nil
}
