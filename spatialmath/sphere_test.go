package spatialmath

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makeTestSphere(point r3.Vector, radius float64, label string) Geometry {
	sphere, _ := NewSphere(point, radius, label)
	return sphere
}

func TestNewSphere(t *testing.T) {
	offset := NewPoseFromOrientation(r3.Vector{X: 1, Y: 0, Z: 0}, &EulerAngles{0, 0, math.Pi})

	// test sphere created from NewBox method
	geometry, err := NewSphere(offset.Point(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometry, test.ShouldResemble, &sphere{pose: NewPoseFromPoint(offset.Point()), radius: 1})
	_, err = NewSphere(offset.Point(), -1, "")
	test.That(t, err.Error(), test.ShouldContainSubstring, newBadGeometryDimensionsError(&sphere{}).Error())

	// test sphere created from GeometryCreator with offset
	gc, err := NewSphereCreator(1, offset, "")
	test.That(t, err, test.ShouldBeNil)
	geometry = gc.NewGeometry(PoseInverse(offset))
	test.That(t, PoseAlmostCoincident(geometry.Pose(), NewZeroPose()), test.ShouldBeTrue)
}

func TestSphereAlmostEqual(t *testing.T) {
	original := makeTestSphere(r3.Vector{}, 1, "")
	good := makeTestSphere(r3.Vector{1e-16, 1e-16, 1e-16}, 1+1e-16, "")
	bad := makeTestSphere(r3.Vector{1e-2, 1e-2, 1e-2}, 1+1e-2, "")
	test.That(t, original.AlmostEqual(good), test.ShouldBeTrue)
	test.That(t, original.AlmostEqual(bad), test.ShouldBeFalse)
}

func TestSphereVertices(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(makeTestSphere(r3.Vector{}, 1, "").Vertices()[0], r3.Vector{}, 1e-8), test.ShouldBeTrue)
}
