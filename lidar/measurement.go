package lidar

import (
	"math"

	"github.com/viamrobotics/robotcore/utils"
)

type Measurements []*Measurement

func (ms Measurements) Len() int {
	return len(ms)
}

func (ms Measurements) Swap(i, j int) {
	ms[i], ms[j] = ms[j], ms[i]
}

func (ms Measurements) Less(i, j int) bool {
	if ms[i].angle < ms[j].angle {
		return true
	}
	if ms[i].angle == ms[j].angle {
		return ms[i].distance < ms[j].distance
	}
	return false
}

type Measurement struct {
	angle    float64
	angleDeg float64
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
		angleDeg: utils.RadToDeg(angle),
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
