package lidar

import (
	"math"
)

type Measurements []*Measurement

type Measurement struct {
	angle    float64
	distance float64
	x        float64
	y        float64
}

func NewMeasurement(angle, distance float64) *Measurement {
	x := distance * math.Cos(angle)
	y := distance * math.Sin(angle)
	return &Measurement{
		angle:    angle,
		distance: distance,
		x:        x,
		y:        y,
	}
}

// in radians
func (m *Measurement) Angle() float64 {
	return m.angle
}

func (m *Measurement) Distance() float64 {
	return m.distance
}

func (m *Measurement) Coords() (float64, float64) {
	return m.x, m.y
}
