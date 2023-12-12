package dualgps

import (
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
)

func TestCreateAndReconfigure(t *testing.T) {

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
