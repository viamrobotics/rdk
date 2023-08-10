package gpsnmea

import (
	"math"
	"testing"

	"go.viam.com/test"
)

func TestParse2(t *testing.T) {
	var data GPSData
	nmeaSentence := "$GBGSV,1,1,01,33,56,045,27,1*40"
	err := data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, data.Speed, test.ShouldAlmostEqual, 0)
	test.That(t, math.IsNaN(data.Location.Lng()), test.ShouldBeTrue)
	test.That(t, math.IsNaN(data.Location.Lat()), test.ShouldBeTrue)

	nmeaSentence = "$GNGLL,4046.43133,N,07358.90383,W,203755.00,A,A*6B"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 40.773855499999996, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -73.9817305, 0.001)

	nmeaSentence = "$GNRMC,203756.00,A,4046.43152,N,07358.90347,W,0.059,,120723,,,A,V*0D"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldAlmostEqual, 0.030351959999999997)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 40.77385866666667, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -73.9817245, 0.001)
	test.That(t, math.IsNaN(data.CompassHeading), test.ShouldBeTrue)

	nmeaSentence = "$GPRMC,210230,A,3855.4487,N,09446.0071,W,0.0,076.2,130495,003.8,E*69"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldAlmostEqual, 0)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 38.924144999999996, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -94.76678500000001, 0.001)
	test.That(t, data.CompassHeading, test.ShouldAlmostEqual, 72.4)

	nmeaSentence = "$GNVTG,,T,,M,0.059,N,0.108,K,A*38"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldEqual, 0.03000024)

	nmeaSentence = "$GNGGA,203756.00,4046.43152,N,07358.90347,W,1,05,4.65,141.4,M,-34.4,M,,*7E"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.valid, test.ShouldBeTrue)
	test.That(t, data.Alt, test.ShouldEqual, 141.4)
	test.That(t, data.SatsInUse, test.ShouldEqual, 5)
	test.That(t, data.HDOP, test.ShouldEqual, 4.65)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 40.77385866666667, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -73.9817245, 0.001)

	nmeaSentence = "$GNGSA,A,3,05,23,15,18,,,,,,,,,5.37,4.65,2.69,1*03"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.HDOP, test.ShouldEqual, 4.65)
	test.That(t, data.VDOP, test.ShouldEqual, 2.69)
}

func TestParsing(t *testing.T) {
	var data GPSData
	// Test a GGA sentence
	nmeaSentence := "$GNGGA,191351.000,4403.4655,N,12118.7950,W,1,6,1.72,1094.5,M,-19.6,M,,*47"
	err := data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.valid, test.ShouldBeTrue)
	test.That(t, data.Alt, test.ShouldEqual, 1094.5)
	test.That(t, data.SatsInUse, test.ShouldEqual, 6)
	test.That(t, data.HDOP, test.ShouldEqual, 1.72)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 44.05776, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -121.31325, 0.001)

	// Test GSA, should update HDOP
	nmeaSentence = "$GPGSA,A,3,21,10,27,08,,,,,,,,,1.98,2.99,0.98*0E"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.HDOP, test.ShouldEqual, 2.99)
	test.That(t, data.VDOP, test.ShouldEqual, 0.98)

	// Test VTG, should update speed
	nmeaSentence = "$GNVTG,176.25,T,,M,0.13,N,0.25,K,A*21"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldEqual, 0.069445)

	// Test RMC, should update speed and position
	nmeaSentence = "$GNRMC,191352.000,A,4503.4656,N,13118.7951,W,0.04,90.29,011021,,,A*59"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldAlmostEqual, 0.0205776)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 45.05776, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, -131.31325, 0.001)

	// Test GSV, should update total sats in view
	nmeaSentence = " $GLGSV,2,2,07,85,23,327,34,70,21,234,21,77,07,028,*50"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.SatsInView, test.ShouldEqual, 7)

	// Test GNS, should update same fields as GGA
	nmeaSentence = "$GNGNS,014035.00,4332.69262,S,17235.48549,E,RR,13,0.9,25.63,11.24,,*70"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.valid, test.ShouldBeTrue)
	test.That(t, data.Alt, test.ShouldEqual, 25.63)
	test.That(t, data.SatsInUse, test.ShouldEqual, 13)
	test.That(t, data.HDOP, test.ShouldEqual, 0.9)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, -43.544877, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, 172.59142, 0.001)

	// Test GLL, should update location
	nmeaSentence = "$GPGLL,4112.26,N,11332.22,E,213276,A,*05"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 41.20433, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, 113.537, 0.001)

	nmeaSentence = "$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A"
	err = data.ParseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.Speed, test.ShouldAlmostEqual, 11.523456)
	test.That(t, data.Location.Lat(), test.ShouldAlmostEqual, 48.117299999, 0.001)
	test.That(t, data.Location.Lng(), test.ShouldAlmostEqual, 11.516666666, 0.001)
	test.That(t, data.CompassHeading, test.ShouldAlmostEqual, 87.5)
}
