package lidar

import (
	"encoding/json"
	"math"

	"go.viam.com/robotcore/utils"

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
	if ms[i].data.Angle < ms[j].data.Angle {
		return true
	}
	if ms[i].data.Angle == ms[j].data.Angle {
		return ms[i].data.Distance < ms[j].data.Distance
	}
	return false
}

type Measurement struct {
	data measurementData
}

type measurementData struct {
	Angle    float64 `json:"angle"`
	AngleDeg float64 `json:"angle_deg"`
	Distance float64 `json:"distance"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

func (m *Measurement) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.data)
}

func (m *Measurement) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.data)
}

func NewMeasurement(angleDegrees, distance float64) *Measurement {
	rad := utils.DegToRad(angleDegrees)
	x, y := utils.RayToUpwardCWCartesian(angleDegrees, distance)
	return &Measurement{
		data: measurementData{
			Angle:    rad,
			AngleDeg: angleDegrees,
			Distance: distance,
			X:        x,
			Y:        y,
		},
	}
}

func (m *Measurement) AngleRad() float64 {
	return m.data.Angle
}

func (m *Measurement) AngleDeg() float64 {
	return m.data.AngleDeg
}

func (m *Measurement) Distance() float64 {
	return m.data.Distance
}

func (m *Measurement) Coords() (float64, float64) {
	return m.data.X, m.data.Y
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

		ms = append(ms, MeasurementFromCoord(x, y))
	}
	return ms
}

func MeasurementFromCoord(x, y float64) *Measurement {
	ang := utils.RadToDeg(math.Atan2(x, y))
	if ang < 0 {
		ang = 360 + ang
	}
	return NewMeasurement(ang, math.Sqrt(x*x+y*y))
}
