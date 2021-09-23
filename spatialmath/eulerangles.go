package spatialmath

import (
	"gonum.org/v1/gonum/num/quat"
)

// EulerAngles are three angles used to represent the rotation of an object in 3D Euclidean space
type EulerAngles struct {
	Roll  float64 `json:"roll"`
	Pitch float64 `json:"pitch"`
	Yaw   float64 `json:"yaw"`
}

// NewEulerAngles creates an empty EulerAngles struct
func NewEulerAngles() *EulerAngles {
	return &EulerAngles{Roll: 0, Pitch: 0, Yaw: 0}
}

func (ea *EulerAngles) EulerAngles() *EulerAngles {
	return ea
}

func (ea *EulerAngles) Quaternion() quat.Number {
	cy := math.Cos(ea.Yaw * 0.5)
	sy := math.Sin(ea.Yaw * 0.5)
	cp := math.Cos(ea.Pitch * 0.5)
	sp := math.Sin(ea.Pitch * 0.5)
	cr := math.Cos(ea.Roll * 0.5)
	sr := math.Sin(ea.Roll * 0.5)

	q := quat.Number{}
	q.Real = cr*cp*cy + sr*sp*sy
	q.Imag = sr*cp*cy - cr*sp*sy
	q.Jmag = cr*sp*cy + sr*cp*sy
	q.Kmag = cr*cp*sy - sr*sp*cy

	return q
}

func (ea *EulerAngles) OrientationVector() *OrientationVec {
	return QuatToOV(ea.Quaternion())
}

func (ea *EulerAngles) OrientationVectorDegrees() *OrientationVecDegrees {
	return QuatToOVD(ea.Quaternion())
}

func (ea *EulerAngles) AxisAngles() *R4AA {
	return &QuatToR4AA(ea.Quaternion())
}
