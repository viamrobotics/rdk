package gpsnmea

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

const (
	knotsToMPerSec = 0.51444
	kphToMPerSec   = 0.27778
)

// GPSData struct combines various attributes related to GPS.
type GPSData struct {
	Location            *geo.Point
	Alt                 float64
	Speed               float64 // ground speed in m per sec
	VDOP                float64 // vertical accuracy
	HDOP                float64 // horizontal accuracy
	SatsInView          int     // quantity satellites in view
	SatsInUse           int     // quantity satellites in view
	valid               bool
	FixQuality          int
	CompassHeading      float64 // true compass heading in degree
	isEast              bool    // direction for magnetic variation which outputs East or West.
	validCompassHeading bool    // true if we get course of direction instead of empty strings.
}

func errInvalidFix(sentenceType, badFix, goodFix string) error {
	return errors.Errorf("type %q sentence fix is not valid have: %q  want %q", sentenceType, badFix, goodFix)
}

// ParseAndUpdate will attempt to parse a line to an NMEA sentence, and if valid, will try to update the given struct
// with the values for that line. Nothing will be updated if there is not a valid gps fix.
func (g *GPSData) ParseAndUpdate(line string) error {
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
	if s.DataType() == nmea.TypeRMC {
		g.parseRMC(line)
	}
	errs = g.updateData(s)

	if g.Location == nil {
		g.Location = geo.NewPoint(math.NaN(), math.NaN())
		errs = multierr.Combine(errs, errors.New("no Location parsed for nmea gps, using default value of lat: NaN, long: NaN"))
		return errs
	}

	return nil
}

// given an NMEA sentense, updateData updates it. An error is returned if any of
// the function calls fails.
func (g *GPSData) updateData(s nmea.Sentence) error {
	var errs error

	switch sentence := s.(type) {
	case nmea.GSV:
		if gsv, ok := s.(nmea.GSV); ok {
			errs = g.updateGSV(gsv)
		}
	case nmea.RMC:
		if rmc, ok := s.(nmea.RMC); ok {
			errs = g.updateRMC(rmc)
		}
	case nmea.GSA:
		if gsa, ok := s.(nmea.GSA); ok {
			errs = g.updateGSA(gsa)
		}
	case nmea.GGA:
		if gga, ok := s.(nmea.GGA); ok {
			errs = g.updateGGA(gga)
		}
	case nmea.GLL:
		if gll, ok := s.(nmea.GLL); ok {
			errs = g.updateGLL(gll)
		}
	case nmea.VTG:
		if vtg, ok := s.(nmea.VTG); ok {
			errs = g.updateVTG(vtg)
		}
	case nmea.GNS:
		if gns, ok := s.(nmea.GNS); ok {
			errs = g.updateGNS(gns)
		}
	case nmea.HDT:
		if hdt, ok := s.(nmea.HDT); ok {
			errs = g.updateHDT(hdt)
		}
	default:
		// Handle the case when the sentence type is not recognized
		errs = fmt.Errorf("unrecognized sentence type: %T", sentence)
	}

	return errs
}

// updateGSV updates g.SatsInView with the information from the provided
// GSV (GPS Satellites in View) data.
//
//nolint:all
func (g *GPSData) updateGSV(gsv nmea.GSV) error {
	// GSV provides the number of satellites in view

	g.SatsInView = int(gsv.NumberSVsInView)
	return nil
}

// updateRMC updates the GPSData object with the information from the provided
// RMC (Recommended Minimum Navigation Information) data.
func (g *GPSData) updateRMC(rmc nmea.RMC) error {
	if rmc.Validity == "A" {
		g.valid = true
	} else if rmc.Validity == "V" {
		g.valid = false
		err := errInvalidFix(rmc.Type, rmc.Validity, "A")
		return err
	}
	if g.valid {
		g.Speed = rmc.Speed * knotsToMPerSec
		g.Location = geo.NewPoint(rmc.Latitude, rmc.Longitude)

		if g.validCompassHeading {
			g.CompassHeading = calculateTrueHeading(rmc.Course, rmc.Variation, g.isEast)
		} else {
			g.CompassHeading = math.NaN()
		}
	}
	return nil
}

// updateGSA updates the GPSData object with the information from the provided
// GSA (GPS DOP and Active Satellites) data.
func (g *GPSData) updateGSA(gsa nmea.GSA) error {
	switch gsa.FixType {
	case "1":
		// No fix
		g.valid = false
		err := errInvalidFix(gsa.Type, gsa.FixType, "1 or 2")
		return err
	case "2":
		// 2d fix, valid lat/lon but invalid Alt
		g.valid = true
		g.VDOP = -1
	case "3":
		// 3d fix
		g.valid = true
	}

	if g.valid {
		g.VDOP = gsa.VDOP
		g.HDOP = gsa.HDOP
	}
	g.SatsInUse = len(gsa.SV)

	return nil
}

// updateGGA updates the GPSData object with the information from the provided
// GGA (Global Positioning System Fix Data) data.
func (g *GPSData) updateGGA(gga nmea.GGA) error {
	var err error

	g.FixQuality, err = strconv.Atoi(gga.FixQuality)
	if err != nil {
		return err
	}

	if gga.FixQuality == "0" {
		g.valid = false
		err = errInvalidFix(gga.Type, gga.FixQuality, "1 to 6")
	} else {
		g.valid = true
		g.Location = geo.NewPoint(gga.Latitude, gga.Longitude)
		g.SatsInUse = int(gga.NumSatellites)
		g.HDOP = gga.HDOP
		g.Alt = gga.Altitude
	}
	return err
}

// updateGLL updates g.Location with the location information from the provided
// GLL (Geographic Position - Latitude/Longitude) data.
//
//nolint:all
func (g *GPSData) updateGLL(gll nmea.GLL) error {
	now := toPoint(gll)
	g.Location = now
	return nil
}

// updateVTG updates g.Speed with the ground speed information from the provided
// VTG (Velocity Made Good) data.
//
//nolint:all
func (g *GPSData) updateVTG(vtg nmea.VTG) error {
	// VTG provides ground speed
	g.Speed = vtg.GroundSpeedKPH * kphToMPerSec
	return nil
}

// updateGNS updates the GPSData object with the information from the provided
// GNS (Global Navigation Satellite System) data.
func (g *GPSData) updateGNS(gns nmea.GNS) error {
	for _, mode := range gns.Mode {
		if mode == "N" {
			g.valid = false
			err := errInvalidFix(gns.Type, mode, " A, D, P, R, F, E, M or S")
			return err
		}
	}

	if g.valid {
		g.Location = geo.NewPoint(gns.Latitude, gns.Longitude)
		g.SatsInUse = int(gns.SVs)
		g.HDOP = gns.HDOP
		g.Alt = gns.Altitude
	}

	return nil
}

// updateHDT updaates g.CompassHeading with the ground speed information from the provided
//
//nolint:all
func (g *GPSData) updateHDT(hdt nmea.HDT) error {
	// HDT provides compass heading
	g.CompassHeading = hdt.Heading
	return nil
}

// calculateTrueHeading is used to get true compass heading from RCM messages.
func calculateTrueHeading(heading, magneticDeclination float64, isEast bool) float64 {
	var adjustment float64
	if isEast {
		adjustment = magneticDeclination
	} else {
		adjustment = -magneticDeclination
	}

	trueHeading := heading + adjustment
	if trueHeading < 0 {
		trueHeading += 360.0
	} else if trueHeading >= 360 {
		trueHeading -= 360.0
	}

	return trueHeading
}

// parseRMC sets g.isEast bool value by parsing the RMC message for compass heading
// and sets g.validCompassHeading bool since RMC message sends empty strings if
// there is no movement.
// go-nmea library does not provide this feature.
func (g *GPSData) parseRMC(message string) {
	data := strings.Split(message, ",")
	if len(data) < 10 {
		return
	}

	if data[8] == "" {
		g.validCompassHeading = false
	} else {
		g.validCompassHeading = true
	}

	// Check if the magnetic declination is East or West
	g.isEast = strings.Contains(data[10], "E")
}
