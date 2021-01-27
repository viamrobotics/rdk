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
	// Remember, our view is from x,y=0,0 at top left
	// of a containing matrix
	// 0°   -  (0,-1) // Up
	// 90°  -  (1, 0) // Right
	// 180° -  (0, 1) // Down
	// 270° -  (-1,0) // Left
	x := distance * math.Sin(angle)
	y := distance * -math.Cos(angle)
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
