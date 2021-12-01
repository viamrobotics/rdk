package lidar

import (
	"encoding/json"
	"math"

	"go.viam.com/core/utils"

	"gonum.org/v1/gonum/mat"
)

// Measurements are a series of lidar measurements in no particular order.
type Measurements []*Measurement

// Len returns the number of measurements present.
func (ms Measurements) Len() int {
	return len(ms)
}

// Swap swaps two measurements positionally.
func (ms Measurements) Swap(i, j int) {
	ms[i], ms[j] = ms[j], ms[i]
}

// Less compares two measurements first by their angles and then by
// their distances if the angels are equal.
func (ms Measurements) Less(i, j int) bool {
	if ms[i].data.Angle < ms[j].data.Angle {
		return true
	}
	if ms[i].data.Angle == ms[j].data.Angle {
		return ms[i].data.Distance < ms[j].data.Distance
	}
	return false
}

// ClosestToDegree returns the measurement, if any, that is closest to
// the given degree.
func (ms Measurements) ClosestToDegree(degree float64) *Measurement {
	var best *Measurement
	bestDiff := 10000.0

	for _, m := range ms {
		diff := utils.AngleDiffDeg(degree, m.data.AngleDeg)
		if diff < bestDiff {
			bestDiff = diff
			best = m
		}
	}

	return best
}

// A Measurement represents a single point detected by a lidar device.
type Measurement struct {
	data measurementData
}

type measurementData struct {
	// Angle is the angle in radians clockwise from the device.
	Angle float64 `json:"angle"`

	// AngleDeg is the angle in degrees clockwise from the device.
	AngleDeg float64 `json:"angle_deg"`

	// Distance is the euclidean distance to the point.
	Distance float64 `json:"distance"`

	// X is the x coordinate of the point.
	X float64 `json:"x"`

	// Y is the y coordinate of the point.
	Y float64 `json:"y"`
}

// MarshalJSON serializes the measurement to JSON.
func (m *Measurement) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.data)
}

// UnmarshalJSON deserializes into the measurement from JSON.
func (m *Measurement) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.data)
}

// NewMeasurement computes a new measurement based on the given angle (degrees)
// and distance.
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

// AngleRad returns the angle in radians clockwise from the device.
func (m *Measurement) AngleRad() float64 {
	return m.data.Angle
}

// AngleDeg returns the angle in degrees clockwise from the device.
func (m *Measurement) AngleDeg() float64 {
	return m.data.AngleDeg
}

// Distance returns the euclidean distance to the point.
func (m *Measurement) Distance() float64 {
	return m.data.Distance
}

// Coords return the Cartesian coordinates of the point.
func (m *Measurement) Coords() (float64, float64) {
	return m.data.X, m.data.Y
}

// MeasurementsFromVec2Matrix is a utility method to decompose a
// a matrix composed of 2D vectors that represent X,Y points into
// a slice of measurements. The function will panic if the matrix
// is malformed.
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

// MeasurementFromCoord takes a Cartesian coordinate and converts
// it into a Measurement.
func MeasurementFromCoord(x, y float64) *Measurement {
	ang := utils.RadToDeg(math.Atan2(x, y))
	if ang < 0 {
		ang = 360 + ang
	}
	return NewMeasurement(ang, math.Sqrt(x*x+y*y))
}
