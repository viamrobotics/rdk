package spatialmath

import (
	"math"
	"testing"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

func TestBasicPoseConstruction(t *testing.T) {

	p := NewEmptyPose()
	// Should return an identity dual quat
	test.That(t, p.Orientation(), test.ShouldResemble, &OrientationVec{0, 0, 0, 1})

	// point at +Y, rotate 90 degrees
	ov := &OrientationVec{math.Pi / 2, 0, 1, 0}
	ov.Normalize()

	p = NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, ov)
	ovCompare(t, p.Orientation(), ov)
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})

	aa := QuatToR4AA(ov.ToQuat())
	p = NewPoseFromAxisAngle(r3.Vector{1, 2, 3}, r3.Vector{aa.RX, aa.RY, aa.RZ}, aa.Theta)
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})
	ovCompare(t, p.Orientation(), ov)

	p = NewPoseFromPoint(r3.Vector{1, 2, 3})
	ptCompare(t, p.Point(), r3.Vector{1, 2, 3})
	test.That(t, p.Orientation(), test.ShouldResemble, &OrientationVec{0, 0, 0, 1})

	p1 := NewPoseFromOrientationVector(r3.Vector{1, 2, 3}, ov)
	p2 := NewPoseFromPoint(r3.Vector{1, 2, 3})
	pComp := Compose(p1, p2)
	ptCompare(t, pComp.Point(), r3.Vector{0, 5, 5})

	p2 = NewPoseFromOrientationVector(r3.Vector{2, 2, 4}, ov)
	delta := PoseDelta(p1, p2)

	test.That(t, delta, test.ShouldResemble, []float64{1, 0, 1, 0, 0, 0})
}

func ptCompare(t *testing.T, p1, p2 r3.Vector) {
	test.That(t, p1.X, test.ShouldAlmostEqual, p2.X)
	test.That(t, p1.Y, test.ShouldAlmostEqual, p2.Y)
	test.That(t, p1.Z, test.ShouldAlmostEqual, p2.Z)
}
