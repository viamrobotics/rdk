package gpsutils

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

// NmeaParser struct combines various attributes related to GPS.
type NmeaParser struct {
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
	GGAMessage          string
}

func errInvalidFix(sentenceType, badFix, goodFix string) error {
	return errors.Errorf("type %q sentence fix is not valid have: %q  want %q", sentenceType, badFix, goodFix)
}

// ParseAndUpdate will attempt to parse a line to an NMEA sentence, and if valid, will try to update the given struct
// with the values for that line. Nothing will be updated if there is not a valid gps fix.
func (g *NmeaParser) ParseAndUpdate(line string) error {
	// Each line should start with a dollar sign and a capital G. If we start reading in the middle
	// of a sentence, though, we'll get unparseable readings. Strip out everything from before the
	// dollar sign, to avoid this confusion.
	ind := strings.Index(line, "$G")
	if ind == -1 {
		line = "" // This line does not contain the start of a message!?
	} else {
		line = line[ind:]
	}

	s, err := nmea.Parse(line)
	if err != nil {
		return err
	}

	// The nmea.RMC message does not support parsing compass heading in its messages. So, we check
	// on that separately, before updating the data we parsed with the third-party package.
	if s.DataType() == nmea.TypeRMC {
		g.parseRMC(line)
	}
	err = g.updateData(s)

	if g.Location == nil {
		g.Location = geo.NewPoint(math.NaN(), math.NaN())
		return multierr.Combine(err, errors.New("no Location parsed for nmea gps, using default value of lat: NaN, long: NaN"))
	}

	return nil
}

// Given an NMEA sentence, updateData updates it. An error is returned if any of
// the function calls fails.
func (g *NmeaParser) updateData(s nmea.Sentence) error {
	switch sentence := s.(type) {
	case nmea.GSV:
		if gsv, ok := s.(nmea.GSV); ok {
			return g.updateGSV(gsv)
		}
	case nmea.RMC:
		if rmc, ok := s.(nmea.RMC); ok {
			return g.updateRMC(rmc)
		}
	case nmea.GSA:
		if gsa, ok := s.(nmea.GSA); ok {
			return g.updateGSA(gsa)
		}
	case nmea.GGA:
		if gga, ok := s.(nmea.GGA); ok {
			return g.updateGGA(gga)
		}
	case nmea.GLL:
		if gll, ok := s.(nmea.GLL); ok {
			return g.updateGLL(gll)
		}
	case nmea.VTG:
		if vtg, ok := s.(nmea.VTG); ok {
			return g.updateVTG(vtg)
		}
	case nmea.GNS:
		if gns, ok := s.(nmea.GNS); ok {
			return g.updateGNS(gns)
		}
	case nmea.HDT:
		if hdt, ok := s.(nmea.HDT); ok {
			return g.updateHDT(hdt)
		}
	default:
		return fmt.Errorf("unrecognized sentence type: %T", sentence)
	}

	return fmt.Errorf("could not cast sentence to expected type: %v", s)
}

// updateGSV updates g.SatsInView with the information from the provided
// GSV (GPS Satellites in View) data.
func (g *NmeaParser) updateGSV(gsv nmea.GSV) error {
	// GSV provides the number of satellites in view

	g.SatsInView = int(gsv.NumberSVsInView)
	return nil
}

// updateRMC updates the NmeaParser object with the information from the provided
// RMC (Recommended Minimum Navigation Information) data.
func (g *NmeaParser) updateRMC(rmc nmea.RMC) error {
	if rmc.Validity != "A" {
		g.valid = false
		return errInvalidFix(rmc.Type, rmc.Validity, "A")
	}

	g.valid = true
	g.Speed = rmc.Speed * knotsToMPerSec
	g.Location = geo.NewPoint(rmc.Latitude, rmc.Longitude)

	if g.validCompassHeading {
		g.CompassHeading = calculateTrueHeading(rmc.Course, rmc.Variation, g.isEast)
	} else {
		g.CompassHeading = math.NaN()
	}
	return nil
}

// updateGSA updates the NmeaParser object with the information from the provided
// GSA (GPS DOP and Active Satellites) data.
func (g *NmeaParser) updateGSA(gsa nmea.GSA) error {
	switch gsa.FixType {
	case "2":
		// 2d fix, valid lat/lon but invalid Alt
		g.valid = true
	case "3":
		// 3d fix
		g.valid = true
	default:
		// No fix
		g.valid = false
		return errInvalidFix(gsa.Type, gsa.FixType, "2 or 3")
	}

	if g.valid {
		g.VDOP = gsa.VDOP
		g.HDOP = gsa.HDOP
	}
	g.SatsInUse = len(gsa.SV)

	return nil
}

// updateGGA updates the NmeaParser object with the information from the provided
// GGA (Global Positioning System Fix Data) data.
func (g *NmeaParser) updateGGA(gga nmea.GGA) error {
	var err error

	g.FixQuality, err = strconv.Atoi(gga.FixQuality)
	if err != nil {
		return err
	}

	if gga.FixQuality == "0" {
		g.valid = false
		return errInvalidFix(gga.Type, gga.FixQuality, "1 to 6")
	}

	g.valid = true
	g.Location = geo.NewPoint(gga.Latitude, gga.Longitude)
	g.SatsInUse = int(gga.NumSatellites)
	g.HDOP = gga.HDOP
	g.Alt = gga.Altitude
	g.GGAMessage = gga.String()
	return nil
}

// updateGLL updates g.Location with the location information from the provided
// GLL (Geographic Position - Latitude/Longitude) data.
func (g *NmeaParser) updateGLL(gll nmea.GLL) error {
	now := geo.NewPoint(gll.Latitude, gll.Longitude)
	g.Location = now
	return nil
}

// updateVTG updates g.Speed with the ground speed information from the provided
// VTG (Velocity Made Good) data.
func (g *NmeaParser) updateVTG(vtg nmea.VTG) error {
	// VTG provides ground speed
	g.Speed = vtg.GroundSpeedKPH * kphToMPerSec

	// Check if the true heading is provided before updating
	if vtg.TrueTrack != 0 {
		// Update the true heading in degrees
		g.CompassHeading = vtg.TrueTrack
	}

	return nil
}

// updateGNS updates the NmeaParser object with the information from the provided
// GNS (Global Navigation Satellite System) data.
func (g *NmeaParser) updateGNS(gns nmea.GNS) error {
	// For each satellite we've heard from, make sure the mode is valid. If any of them are not
	// valid, this entire message should not be trusted.
	for _, mode := range gns.Mode {
		if mode == "N" { // No satellite fix
			g.valid = false
			return errInvalidFix(gns.Type, mode, "A, D, P, R, F, E, M or S")
		}
	}

	if !g.valid { // This value gets set elsewhere, such as in a GGA message.
		return nil // Don't parse this message; we're not set up yet.
	}

	g.Location = geo.NewPoint(gns.Latitude, gns.Longitude)
	g.SatsInUse = int(gns.SVs)
	g.HDOP = gns.HDOP
	g.Alt = gns.Altitude
	return nil
}

// updateHDT updates g.CompassHeading with the ground speed information from the provided.
func (g *NmeaParser) updateHDT(hdt nmea.HDT) error {
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
// NOTE: The go-nmea library does not provide this feature, which is why we're doing it ourselves.
func (g *NmeaParser) parseRMC(message string) {
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
