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

func (m *ForceVector) SetZero() {
	m.Moment = mgl64.Vec3{0, 0, 0}
	m.Force = mgl64.Vec3{0, 0, 0}
}
