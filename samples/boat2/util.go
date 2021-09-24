package main

import (
	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
)

// ToPoint converts a nmea.GLL to a geo.Point
func ToPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
