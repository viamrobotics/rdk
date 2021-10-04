package nmea

import (
	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
)

const knotsToKph = 1.852

type gpsData struct {
	location   *geo.Point
	alt        float64
	speed      float64
	vDOP       float64 // vertical accuracy
	hDOP       float64 // horizontal accuracy
	satsInView int     // quantity satellites in view
	satsInUse  int     // quantity satellites in view
	valid      bool
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
			g.speed = rmc.Speed * knotsToKph
			g.location = geo.NewPoint(rmc.Latitude, rmc.Longitude)
		}
	} else if gsa, ok := s.(nmea.GSA); ok {
		// GSA gives horizontal and vertical accuracy, and also describes the type of lock- invalid, 2d, or 3d.
		if gsa.FixType == "1" {
			// No fix
			g.valid = false
		} else if gsa.FixType == "2" {
			// 2d fix, valid lat/lon but invalid alt
			g.valid = true
			g.vDOP = -1
		} else if gsa.FixType == "3" {
			// 3d fix
			g.valid = true
		}
		if g.valid {
			g.vDOP = gsa.VDOP
			g.hDOP = gsa.HDOP
		}
		g.satsInUse = len(gsa.SV)
	} else if gga, ok := s.(nmea.GGA); ok {
		// GGA provides validity, lon/lat, altitude, altitude, sats in use, and horizontal position error
		if gga.FixQuality == "0" {
			g.valid = false
		} else {
			g.valid = true
			g.location = geo.NewPoint(gga.Latitude, gga.Longitude)
			g.satsInUse = int(gga.NumSatellites)
			g.hDOP = gga.HDOP
			g.alt = gga.Altitude
		}
	} else if gll, ok := s.(nmea.GLL); ok {
		// GLL provides just lat/lon
		now := toPoint(gll)
		g.location = now
	} else if vtg, ok := s.(nmea.VTG); ok {
		// VTG provides ground speed
		g.speed = vtg.GroundSpeedKPH
	} else if gns, ok := s.(nmea.GNS); ok {
		// GNS Provides approximately the same information as GGA
		for _, mode := range gns.Mode {
			if mode == "N" {
				g.valid = false
			}
		}
		if g.valid {
			g.location = geo.NewPoint(gns.Latitude, gns.Longitude)
			g.satsInUse = int(gns.SVs)
			g.hDOP = gns.HDOP
			g.alt = gns.Altitude
		}
	}
	return nil
}
