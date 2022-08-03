package nmea

import (
	"testing"

	"go.viam.com/test"
)

func TestParsing(t *testing.T) {
	var data gpsData
	// Test a GGA sentence
	nmeaSentence := "$GNGGA,191351.000,4403.4655,N,12118.7950,W,1,6,1.72,1094.5,M,-19.6,M,,*47"
	err := data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.valid, test.ShouldBeTrue)
	test.That(t, data.alt, test.ShouldEqual, 1094.5)
	test.That(t, data.satsInUse, test.ShouldEqual, 6)
	test.That(t, data.hDOP, test.ShouldEqual, 1.72)
	test.That(t, data.location.Lat(), test.ShouldAlmostEqual, 44.05776, 0.001)
	test.That(t, data.location.Lng(), test.ShouldAlmostEqual, -121.31325, 0.001)

	// Test GSA, should update HDOP
	nmeaSentence = "$GPGSA,A,3,21,10,27,08,,,,,,,,,1.98,2.99,0.98*0E"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.hDOP, test.ShouldEqual, 2.99)
	test.That(t, data.vDOP, test.ShouldEqual, 0.98)

	// Test VTG, should update speed
	nmeaSentence = "$GNVTG,176.25,T,,M,0.13,N,0.25,K,A*21"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.speed, test.ShouldEqual, 69.445)

	// Test RMC, should update speed and position
	nmeaSentence = "$GNRMC,191352.000,A,4503.4656,N,13118.7951,W,0.04,90.29,011021,,,A*59"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.speed, test.ShouldAlmostEqual, 20.5776)
	test.That(t, data.location.Lat(), test.ShouldAlmostEqual, 45.05776, 0.001)
	test.That(t, data.location.Lng(), test.ShouldAlmostEqual, -131.31325, 0.001)

	// Test GSV, should update total sats in view
	nmeaSentence = " $GLGSV,2,2,07,85,23,327,34,70,21,234,21,77,07,028,*50"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.satsInView, test.ShouldEqual, 7)

	// Test GNS, should update same fields as GGA
	nmeaSentence = "$GNGNS,014035.00,4332.69262,S,17235.48549,E,RR,13,0.9,25.63,11.24,,*70"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.valid, test.ShouldBeTrue)
	test.That(t, data.alt, test.ShouldEqual, 25.63)
	test.That(t, data.satsInUse, test.ShouldEqual, 13)
	test.That(t, data.hDOP, test.ShouldEqual, 0.9)
	test.That(t, data.location.Lat(), test.ShouldAlmostEqual, -43.544877, 0.001)
	test.That(t, data.location.Lng(), test.ShouldAlmostEqual, 172.59142, 0.001)

	// Test GLL, should update location
	nmeaSentence = "$GPGLL,4112.26,N,11332.22,E,213276,A,*05"
	err = data.parseAndUpdate(nmeaSentence)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data.location.Lat(), test.ShouldAlmostEqual, 41.20433, 0.001)
	test.That(t, data.location.Lng(), test.ShouldAlmostEqual, 113.537, 0.001)
}
