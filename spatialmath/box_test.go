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

func TestBoxPC(t *testing.T) {
	offset1 := r3.Vector{2, 2, 0}
	dims1 := r3.Vector{2, 2, 2}
	orien1 := &Quaternion{1, 1, 0, 0}
	pose1 := NewPoseFromOrientation(offset1, orien1)
	box1 := &box{pose1, [3]float64{0.5 * dims1.X, 0.5 * dims1.Y, 0.5 * dims1.Z}, 10, ""} // with abitrary radius bounding sphere
	customDensity := 1.
	output1 := box1.ToPointCloud(customDensity)
	checkAgainst1 := []r3.Vector{
		r3.Vector{4, 3, -2},
		r3.Vector{4, 5, 2},
		r3.Vector{4, 5, -3},
		r3.Vector{4, 1, -1},
		r3.Vector{4, 3, 3},
		r3.Vector{4, 7, 1},
		r3.Vector{5, 3, -2},
		r3.Vector{3, 3, -2},
		r3.Vector{3, 5, 2},
		r3.Vector{5, 5, 2},
		r3.Vector{5, 5, -3},
		r3.Vector{5, 1, -1},
		r3.Vector{3, 5, -3},
		r3.Vector{3, 1, -1},
		r3.Vector{3, 3, 3},
		r3.Vector{3, 7, 1},
		r3.Vector{5, 3, 3},
		r3.Vector{5, 7, 1},
		r3.Vector{5, 4, 0},
		r3.Vector{3, 4, 0},
		r3.Vector{5, 6, -1},
		r3.Vector{5, 2, 1},
		r3.Vector{3, 6, -1},
		r3.Vector{3, 2, 1},
		r3.Vector{4, 6, -1},
		r3.Vector{4, 2, 1},
	}
	for i, v := range output1 {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst1[i], 1e-2), test.ShouldBeTrue)
	}

	offset2 := r3.Vector{2, 2, 2}
	dims2 := r3.Vector{1, 1.5, 4}
	orien2 := &Quaternion{1, 0, 1, 0}
	pose2 := NewPoseFromOrientation(offset2, orien2)
	box2 := &box{pose2, [3]float64{0.5 * dims2.X, 0.5 * dims2.Y, 0.5 * dims2.Z}, 10, ""} // with abitrary radius bounding sphere
	output2 := box2.ToPointCloud(customDensity)
	checkAgainst2 := []r3.Vector{
		r3.Vector{4, 4.75, 4},
		r3.Vector{4, 3.25, 4},
		r3.Vector{2, 4.75, 3},
		r3.Vector{6, 4.75, 5},
		r3.Vector{6, 3.25, 5},
		r3.Vector{2, 3.25, 3},
		r3.Vector{0, 4.75, 2},
		r3.Vector{8, 4.75, 6},
		r3.Vector{8, 3.25, 6},
		r3.Vector{0, 3.25, 2},
		r3.Vector{3.5, 4, 5},
		r3.Vector{4.5, 4, 3},
		r3.Vector{1.5, 4, 4},
		r3.Vector{5.5, 4, 6},
		r3.Vector{2.5, 4, 2},
		r3.Vector{6.5, 4, 4},
		r3.Vector{-0.5, 4, 3},
		r3.Vector{7.5, 4, 7},
		r3.Vector{0.5, 4, 1},
		r3.Vector{8.5, 4, 5},
		r3.Vector{0, 4, 2},
		r3.Vector{8, 4, 6},
	}
	for i, v := range output2 {
		test.That(t, R3VectorAlmostEqual(v, checkAgainst2[i], 1e-2), test.ShouldBeTrue)
	}
}
