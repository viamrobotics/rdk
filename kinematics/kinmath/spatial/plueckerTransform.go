package spatial

import (
	"github.com/go-gl/mathgl/mgl64"
)

type PlueckerTransform struct {
	Translation mgl64.Vec3
	Rotation    mgl64.Mat3
}

func (p PlueckerTransform) Linear() mgl64.Mat3 {
	return p.Rotation
}

// Mult will multiply this PlueckerTransform with another
func (p PlueckerTransform) Mult(p2 PlueckerTransform) PlueckerTransform {
	ret := PlueckerTransform{}
	ret.Rotation = p.Rotation.Mul3(p2.Rotation)
	ret.Translation = p2.Translation.Add(p2.Rotation.Transpose().Mul3x1(p.Translation))
	return ret
}

// MultMV will multiply this PlueckerTransform with a MotionVector
func (p PlueckerTransform) MultMV(m MotionVector) MotionVector {
	ret := MotionVector{}
	ret.Angular = p.Rotation.Transpose().Mul3x1(m.Angular)
	ret.Linear = p.Rotation.Transpose().Mul3x1(m.Linear.Sub(p.Translation.Cross(m.Angular)))
	return ret
}
