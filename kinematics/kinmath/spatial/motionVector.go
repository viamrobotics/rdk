package spatial

import (
	"github.com/go-gl/mathgl/mgl64"
)

type MotionVector struct {
	Angular mgl64.Vec3
	Linear  mgl64.Vec3
}

func NewMVFromVecN(vec *mgl64.VecN) *MotionVector {
	v1 := mgl64.Vec3{vec.Get(0), vec.Get(1), vec.Get(2)}
	v2 := mgl64.Vec3{vec.Get(3), vec.Get(4), vec.Get(5)}
	return &MotionVector{v1, v2}
}

func (m *MotionVector) Cross(other ForceVector) ForceVector {
	var res ForceVector
	res.Moment = m.Angular.Cross(other.Moment).Add(m.Linear.Cross(other.Force))
	res.Force = m.Angular.Cross(other.Force)
	return res
}

func (m *MotionVector) Dot(other ForceVector) float64 {
	return m.Angular.Dot(other.Moment) + m.Linear.Dot(other.Force)
}

func (m *MotionVector) AddMV(other *MotionVector) {
	m.Angular = m.Angular.Add(other.Angular)
	m.Linear = m.Linear.Add(other.Linear)
}

func (m *MotionVector) SetZero() {
	m.Angular = mgl64.Vec3{0, 0, 0}
	m.Linear = mgl64.Vec3{0, 0, 0}
}
