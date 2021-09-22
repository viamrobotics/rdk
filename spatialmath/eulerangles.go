package spatialmath

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
