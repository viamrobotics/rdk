package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeBox(o Orientation, point, dims r3.Vector) *box {
	vc, _ := NewBox(dims, NewPoseFromOrientation(point, o))
	return vc.NewVolume(NewZeroPose()).(*box)
}

func TestNewBoxFromOffset(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})
	vc, err := NewBox(r3.Vector{1, 1, 1}, offset)
	test.That(t, err, test.ShouldBeNil)
	vol := vc.NewVolume(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(vol.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestBoxAlmostEqual(t *testing.T) {
	original := makeBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{1, 1, 1})
	good := makeBox(NewZeroOrientation(), r3.Vector{1e-16, 1e-16, 1e-16}, r3.Vector{1 + 1e-16, 1 + 1e-16, 1 + 1e-16})
	bad := makeBox(NewZeroOrientation(), r3.Vector{1e-2, 1e-2, 1e-2}, r3.Vector{1 + 1e-2, 1 + 1e-2, 1 + 1e-2})
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestBoxVertices(t *testing.T) {
	offset := r3.Vector{2, 2, 2}
	box := makeBox(NewZeroOrientation(), offset, r3.Vector{2, 2, 2})
	vertices := box.Vertices()
	test.That(t, R3VectorAlmostEqual(vertices[0], r3.Vector{1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[1], r3.Vector{1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[2], r3.Vector{1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[3], r3.Vector{1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[4], r3.Vector{-1, 1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[5], r3.Vector{-1, 1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[6], r3.Vector{-1, -1, 1}.Add(offset), 1e-8), test.ShouldBeTrue)
	test.That(t, R3VectorAlmostEqual(vertices[7], r3.Vector{-1, -1, -1}.Add(offset), 1e-8), test.ShouldBeTrue)
}
