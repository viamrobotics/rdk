package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeSphere(point r3.Vector, radius float64) *sphere {
	vc, _ := NewSphere(radius, NewPoseFromPoint(point))
	return vc.NewVolume(NewZeroPose()).(*sphere)
}

func TestNewSphereFromOffset(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})
	vc, err := NewSphere(1, offset)
	test.That(t, err, test.ShouldBeNil)
	vol := vc.NewVolume(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestSphereAlmostEqual(t *testing.T) {
	original := makeSphere(r3.Vector{}, 1)
	good := makeSphere(r3.Vector{1e-16, 1e-16, 1e-16}, 1+1e-16)
	bad := makeSphere(r3.Vector{1e-2, 1e-2, 1e-2}, 1+1e-2)
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestSphereVertices(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(makeSphere(r3.Vector{}, 1).Vertices()[0], r3.Vector{}, 1e-8), test.ShouldBeTrue)
}
