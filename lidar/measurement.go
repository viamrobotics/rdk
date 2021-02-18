package lidar

import (
	"math"

	"github.com/viamrobotics/robotcore/utils"

	"gonum.org/v1/gonum/mat"
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
	rad := utils.DegToRad(angle)
	x := distance * math.Sin(rad)
	y := distance * -math.Cos(rad)
	return &Measurement{
		angle:    rad,
		angleDeg: angle,
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

func MeasurementsFromVec2Matrix(m *utils.Vec2Matrix) Measurements {
	mD := (*mat.Dense)(m)
	if mD.IsEmpty() {
		return nil
	}
	_, c := mD.Dims()
	ms := make(Measurements, 0, c)
	for i := 0; i < c; i++ {
		x := mD.At(0, i)
		y := mD.At(1, i)

		ang := utils.RadToDeg(math.Atan2(x, -y))
		if ang < 0 {
			ang = 360 + ang
		}
		ms = append(ms, NewMeasurement(ang, math.Sqrt(x*x+y*y)))
	}
	return ms
}
