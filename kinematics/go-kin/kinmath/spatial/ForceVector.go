package spatial

import (
	"github.com/go-gl/mathgl/mgl64"
)

type ForceVector struct {
	Moment mgl64.Vec3
	Force  mgl64.Vec3
}

func (f *ForceVector) Dot(other MotionVector) float64 {
	return f.Moment.Dot(other.Angular) + f.Force.Dot(other.Linear)
}

func (f *ForceVector) SetZero() {
	f.Moment = mgl64.Vec3{0, 0, 0}
	f.Force = mgl64.Vec3{0, 0, 0}
}
