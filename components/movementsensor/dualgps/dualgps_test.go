package dualgps

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
)

const (
	testName = "test"
	testGPS1 = "gps1"
	testGPS2 = "gps2"
	testGPS3 = "gps3"
	testPath = "somepath"
)

func TestCreateValidateAndReconfigure(t *testing.T) {
	cfg := resource.Config{
		Name:  testName,
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			Gps1: testGPS1,
		},
	}
	implicits, err := cfg.Validate(testPath, movementsensor.API.SubtypeName)
	test.That(t, implicits, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError(testPath, "second_gps"))

	cfg = resource.Config{
		Name:  testName,
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			Gps1: testGPS1,
			Gps2: testGPS2,
		},
	}
	implicits, err = cfg.Validate(testPath, movementsensor.API.SubtypeName)
	test.That(t, implicits, test.ShouldResemble, []string{testGPS1, testGPS2})
	test.That(t, err, test.ShouldBeNil)

	deps := make(resource.Dependencies)
	deps[movementsensor.Named(testGPS1)] = inject.NewMovementSensor(testGPS1)
	deps[movementsensor.Named(testGPS2)] = inject.NewMovementSensor(testGPS2)
	deps[movementsensor.Named(testGPS3)] = inject.NewMovementSensor(testGPS3)

	ms, err := newDualGPS(
		context.Background(),
		deps,
		cfg,
		logging.NewDebugLogger("testLogger"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)
	dgps, ok := ms.(*dualGPS)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, dgps.gps2.gps.Name().ShortName(), test.ShouldResemble, testGPS2)

	cfg = resource.Config{
		Name:  testName,
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			Gps1: testGPS1,
			Gps2: testGPS3,
		},
	}
	err = ms.Reconfigure(context.Background(), deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	dgps, ok = ms.(*dualGPS)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, dgps.gps2.gps.Name().ShortName(), test.ShouldResemble, testGPS3)

}

func TestGetHeading(t *testing.T) {
	testPos1 := geo.NewPoint(8.46696, -17.03663)
	testPos2 := geo.NewPoint(65.35996, -17.03663)
	// test case 1, standard bearing = 0, heading = 270
	bearing, heading, standardBearing := getHeading(testPos1, testPos2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 0)
	test.That(t, heading, test.ShouldAlmostEqual, 270)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 0)

	// test case 2, reversed test case 1.
	testPos1 = geo.NewPoint(65.35996, -17.03663)
	testPos2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = getHeading(testPos1, testPos2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 180)
	test.That(t, heading, test.ShouldAlmostEqual, 90)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 180)

	// test case 2.5, changed yaw offsets
	testPos1 = geo.NewPoint(65.35996, -17.03663)
	testPos2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = getHeading(testPos1, testPos2, 270)
	test.That(t, bearing, test.ShouldAlmostEqual, 180)
	test.That(t, heading, test.ShouldAlmostEqual, 270)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 180)

	// test case 3
	testPos1 = geo.NewPoint(8.46696, -17.03663)
	testPos2 = geo.NewPoint(56.74367734077241, 29.369620000000015)

	bearing, heading, standardBearing = getHeading(testPos1, testPos2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 27.2412, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 297.24126, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 27.24126, 1e-3)

	// test case 4, reversed coordinates
	testPos1 = geo.NewPoint(56.74367734077241, 29.369620000000015)
	testPos2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = getHeading(testPos1, testPos2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 235.6498, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 145.6498, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, -124.3501, 1e-3)

	// test case 4.5, changed yaw Offset
	testPos1 = geo.NewPoint(56.74367734077241, 29.369620000000015)
	testPos2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = getHeading(testPos1, testPos2, 270)
	test.That(t, bearing, test.ShouldAlmostEqual, 235.6498, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 325.6498, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, -124.3501, 1e-3)
}
