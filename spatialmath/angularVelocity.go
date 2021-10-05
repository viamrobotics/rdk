package spatialmath

// AngularVelocity contains angular velocity in deg/s across x/y/z axes.
type AngularVelocity struct {
	x float64 `json:"x"`
	y float64 `json:"y"`
	z float64 `json:"z"`
}
