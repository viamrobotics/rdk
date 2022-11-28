package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestBox(o Orientation, point, dims r3.Vector, label string) Geometry {
	box, _ := NewBox(NewPoseFromOrientation(point, o), dims, label)
	return box
}

func TestNewBox(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test box created from NewBox method
	geometry, err := NewBox(offset, r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &box{pose: offset, halfSize: [3]float64{0.5, 0.5, 0.5}, boundingSphereR: math.Sqrt(0.75)})
	_, err = NewBox(offset, r3.Vector{-1, 0, 0}, "")
	test.That(t, err.Error(), test.ShouldContainSubstring, newBadGeometryDimensionsError(&box{}).Error())

	// test box created from GeometryCreator with offset
	gc, err := NewBoxCreator(r3.Vector{1, 1, 1}, offset, "")
	test.That(t, err, test.ShouldBeNil)
	geometry = gc.NewGeometry(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestBoxAlmostEqual(t *testing.T) {
	original := makeTestBox(NewZeroOrientation(), r3.Vector{}, r3.Vector{1, 1, 1}, "")
	good := makeTestBox(NewZeroOrientation(), r3.Vector{1e-16, 1e-16, 1e-16}, r3.Vector{1 + 1e-16, 1 + 1e-16, 1 + 1e-16}, "")
	bad := makeTestBox(NewZeroOrientation(), r3.Vector{1e-2, 1e-2, 1e-2}, r3.Vector{1 + 1e-2, 1 + 1e-2, 1 + 1e-2}, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestBoxVertices(t *testing.T) {
	offset := r3.Vector{2, 2, 2}
	box := makeTestBox(NewZeroOrientation(), offset, r3.Vector{2, 2, 2}, "")
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
