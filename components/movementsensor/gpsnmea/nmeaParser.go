package gpsnmea

import (
	"strconv"
	"strings"

	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

const (
	knotsToMmPerSec = 514.44
	kphToMmPerSec   = 277.78
)

type gpsData struct {
	location   *geo.Point
	alt        float64
	speed      float64 // ground speed in mm per sec
	vDOP       float64 // vertical accuracy
	hDOP       float64 // horizontal accuracy
	satsInView int     // quantity satellites in view
	satsInUse  int     // quantity satellites in view
	valid      bool
	fixQuality int
}

func errInvalidFix(sentenceType, badFix, goodFix string) error {
	return errors.Errorf("type %q sentence fix is not valid have: %q  want %q", sentenceType, badFix, goodFix)
}

// parseAndUpdate will attempt to parse a line to an NMEA sentence, and if valid, will try to update the given struct
// with the values for that line. Nothing will be updated if there is not a valid gps fix.
func (g *gpsData) parseAndUpdate(line string) error {
	// add parsing to filter out corrupted data
	ind := strings.Index(line, "$G")
	if ind == -1 {
		line = ""
	} else {
		line = line[ind:]
	}

	var errs error
	s, err := nmea.Parse(line)
	if err != nil {
		return multierr.Combine(errs, err)
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
			errs = multierr.Combine(errs, errInvalidFix(rmc.Type, rmc.Validity, "A"))
		}
		if g.valid {
			g.speed = rmc.Speed * knotsToMmPerSec
			g.location = geo.NewPoint(rmc.Latitude, rmc.Longitude)
		}
	} else if gsa, ok := s.(nmea.GSA); ok {
		// GSA gives horizontal and vertical accuracy, and also describes the type of lock- invalid, 2d, or 3d.
		switch gsa.FixType {
		case "1":
			// No fix
			g.valid = false
			errs = multierr.Combine(errs, errInvalidFix(gsa.Type, gsa.FixType, "1 or 2"))
		case "2":
			// 2d fix, valid lat/lon but invalid alt
			g.valid = true
			g.vDOP = -1
		case "3":
			// 3d fix
			g.valid = true
		}
		if g.valid {
			g.vDOP = gsa.VDOP
			g.hDOP = gsa.HDOP
		}
		g.satsInUse = len(gsa.SV)
	} else if gga, ok := s.(nmea.GGA); ok {
		// GGA provides validity, lon/lat, altitude, sats in use, and horizontal position error
		g.fixQuality, err = strconv.Atoi(gga.FixQuality)
		if err != nil {
			return err
		}
		if gga.FixQuality == "0" {
			g.valid = false
			errs = multierr.Combine(errs, errInvalidFix(gga.Type, gga.FixQuality, "1 to 6"))
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
		g.speed = vtg.GroundSpeedKPH * kphToMmPerSec
	} else if gns, ok := s.(nmea.GNS); ok {
		// GNS Provides approximately the same information as GGA
		for _, mode := range gns.Mode {
			if mode == "N" {
				g.valid = false
				errs = multierr.Combine(errs, errInvalidFix(gns.Type, mode, " A, D, P, R, F, E, M or S"))
			}
		}
		if g.valid {
			g.location = geo.NewPoint(gns.Latitude, gns.Longitude)
			g.satsInUse = int(gns.SVs)
			g.hDOP = gns.HDOP
			g.alt = gns.Altitude
		}
	}

	if g.location == nil {
		g.location = geo.NewPoint(0, 0)
		errs = multierr.Combine(errs, errors.New("no location parsed for nmea gps, using default value of lat:0, long: 0"))
		return errs
	}
	return nil
}
