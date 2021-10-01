package nmea

import (
	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
)

const knotsToKph = 1.852

type gpsData struct {
	lastLocation *geo.Point
	lastAlt      float64
	lastSpeed    float64
	lastVDOP     float64 // vertical accuracy
	lastHDOP     float64 // horizontal accuracy
	satsInView   int     // quantity satellites in view
	satsInUse    int     // quantity satellites in view
	valid        bool
}

// parseAndUpdate will attempt to parse a line to an NMEA sentence, and if valid, will try to update the given struct
// with the values for that line. Nothing will be updated if there is not a valid gps fix.
func parseAndUpdate(line string, g *gpsData) error {
	s, err := nmea.Parse(line)
	if err != nil {
		return err
	}
	// Most receivers support at least the following sentence types: GSV, RMC, GSA, GGA, GLL, VTG, GNS
	if gsv, ok := s.(nmea.GSV); ok {
		// GSV provides the number of satellites in view
		g.satsInView = int(gsv.NumberSVsInView)
	} else if rmc, ok := s.(nmea.RMC); ok {
		// RMC provides validity, lon/lat, and ground speed.
		if rmc.Validity == "A" {
			g.valid = true
		} else if rmc.Validity == "V" {
			g.valid = false
		}
		if g.valid {
			g.lastSpeed = rmc.Speed * knotsToKph
			g.lastLocation = geo.NewPoint(rmc.Latitude, rmc.Longitude)
		}
	} else if gsa, ok := s.(nmea.GSA); ok {
		// GSA gives horizontal and vertical accuracy, and also describes the type of lock- invalid, 2d, or 3d.
		if gsa.FixType == "1" {
			// No fix
			g.valid = false
		} else if gsa.FixType == "2" {
			// 2d fix, valid lat/lon but invalid alt
			g.valid = true
			g.lastVDOP = -1
		} else if gsa.FixType == "3" {
			// 3d fix
			g.valid = true
		}
		if g.valid {
			g.lastVDOP = gsa.VDOP
			g.lastHDOP = gsa.HDOP
		}
		g.satsInUse = len(gsa.SV)
	} else if gga, ok := s.(nmea.GGA); ok {
		// GGA provides validity, lon/lat, altitude, altitude, sats in use, and horizontal position error
		if gga.FixQuality == "0" {
			g.valid = false
		} else {
			g.valid = true
			g.lastLocation = geo.NewPoint(gga.Latitude, gga.Longitude)
			g.satsInUse = int(gga.NumSatellites)
			g.lastHDOP = gga.HDOP
			g.lastAlt = gga.Altitude
		}
	} else if gll, ok := s.(nmea.GLL); ok {
		// GLL provides just lat/lon
		now := toPoint(gll)
		g.lastLocation = now
	} else if vtg, ok := s.(nmea.VTG); ok {
		// VTG provides ground speed
		g.lastSpeed = vtg.GroundSpeedKPH
	} else if gns, ok := s.(nmea.GNS); ok {
		// GNS Provides approximately the same information as GGA
		for _, mode := range gns.Mode {
			if mode == "N" {
				g.valid = false
			}
		}
		if g.valid {
			g.lastLocation = geo.NewPoint(gns.Latitude, gns.Longitude)
			g.satsInUse = int(gns.SVs)
			g.lastHDOP = gns.HDOP
			g.lastAlt = gns.Altitude
		}
	}
	return nil
}
