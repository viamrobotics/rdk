package movementsensor

import (
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
)

func TestGetHeading(t *testing.T) {
	// test case 1, standard bearing = 0, heading = 270
	var (
		GPS1 = geo.NewPoint(8.46696, -17.03663)
		GPS2 = geo.NewPoint(65.35996, -17.03663)
	)

	bearing, heading, standardBearing := GetHeading(GPS1, GPS2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 0)
	test.That(t, heading, test.ShouldAlmostEqual, 270)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 0)

	// test case 2, reversed test case 1.
	GPS1 = geo.NewPoint(65.35996, -17.03663)
	GPS2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = GetHeading(GPS1, GPS2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 180)
	test.That(t, heading, test.ShouldAlmostEqual, 90)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 180)

	// test case 2.5, changed yaw offsets
	GPS1 = geo.NewPoint(65.35996, -17.03663)
	GPS2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = GetHeading(GPS1, GPS2, 270)
	test.That(t, bearing, test.ShouldAlmostEqual, 180)
	test.That(t, heading, test.ShouldAlmostEqual, 270)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 180)

	// test case 3
	GPS1 = geo.NewPoint(8.46696, -17.03663)
	GPS2 = geo.NewPoint(56.74367734077241, 29.369620000000015)

	bearing, heading, standardBearing = GetHeading(GPS1, GPS2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 27.2412, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 297.24126, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, 27.24126, 1e-3)

	// test case 4, reversed coordinates
	GPS1 = geo.NewPoint(56.74367734077241, 29.369620000000015)
	GPS2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = GetHeading(GPS1, GPS2, 90)
	test.That(t, bearing, test.ShouldAlmostEqual, 235.6498, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 145.6498, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, -124.3501, 1e-3)

	// test case 4.5, changed yaw Offset
	GPS1 = geo.NewPoint(56.74367734077241, 29.369620000000015)
	GPS2 = geo.NewPoint(8.46696, -17.03663)

	bearing, heading, standardBearing = GetHeading(GPS1, GPS2, 270)
	test.That(t, bearing, test.ShouldAlmostEqual, 235.6498, 1e-3)
	test.That(t, heading, test.ShouldAlmostEqual, 325.6498, 1e-3)
	test.That(t, standardBearing, test.ShouldAlmostEqual, -124.3501, 1e-3)
}
