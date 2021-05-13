package spatial

import (
	"github.com/go-gl/mathgl/mgl64"
)

// ForceVector TODO
type ForceVector struct {
	Moment mgl64.Vec3
	Force  mgl64.Vec3
}

// Dot TODO
func (f *ForceVector) Dot(other MotionVector) float64 {
	return f.Moment.Dot(other.Angular) + f.Force.Dot(other.Linear)
}

// SetZero TODO
func (f *ForceVector) SetZero() {
	f.Moment = mgl64.Vec3{0, 0, 0}
	f.Force = mgl64.Vec3{0, 0, 0}
}
