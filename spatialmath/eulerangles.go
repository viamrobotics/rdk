package spatialmath

// EulerAngles are three angles used to represent the rotation of an object in 3D Euclidean space
type EulerAngles struct {
	Roll  float64 `json:"roll"`
	Pitch float64 `json:"pitch"`
	Yaw   float64 `json:"yaw"`
}
