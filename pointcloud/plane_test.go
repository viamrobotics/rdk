package pointcloud

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestEmptyPlane(t *testing.T) {
	plane := NewEmptyPlane()
	test.That(t, plane.Equation(), test.ShouldResemble, [4]float64{})
	test.That(t, plane.Normal(), test.ShouldResemble, r3.Vector{})
	test.That(t, plane.Center(), test.ShouldResemble, r3.Vector{})
	test.That(t, plane.Offset(), test.ShouldEqual, 0.0)
	cloud, err := plane.PointCloud()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloud, test.ShouldNotBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 0)
	pt := r3.Vector{1, 2, 3}
	test.That(t, plane.Distance(pt), test.ShouldEqual, 0)
}

func TestNewPlane(t *testing.T) {
	// make the point cloud, a diamond of slope 1 in x and y
	pc := New()
	p0 := NewVector(0., 0., 0.)
	test.That(t, pc.Set(p0, nil), test.ShouldBeNil)
	p1 := NewVector(0., 2., 2.)
	test.That(t, pc.Set(p1, nil), test.ShouldBeNil)
	p2 := NewVector(2., 0., 2.)
	test.That(t, pc.Set(p2, nil), test.ShouldBeNil)
	p3 := NewVector(2., 2., 4.)
	test.That(t, pc.Set(p3, nil), test.ShouldBeNil)
	eq := [4]float64{1, 1, -1, 0}
	// run the tests
	plane := NewPlane(pc, eq)
	test.That(t, plane.Equation(), test.ShouldResemble, eq)
	test.That(t, plane.Normal(), test.ShouldResemble, r3.Vector{1, 1, -1})
	test.That(t, plane.Center(), test.ShouldResemble, r3.Vector{1, 1, 2})
	test.That(t, plane.Offset(), test.ShouldEqual, 0.0)
	cloud, err := plane.PointCloud()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloud, test.ShouldNotBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 4)
	pt := r3.Vector{-1, -1, 1}
	test.That(t, math.Abs(plane.Distance(pt)), test.ShouldAlmostEqual, math.Sqrt(3))
}

func TestIntersect(t *testing.T) {
	// plane at z = 0
	plane := NewPlane(nil, [4]float64{0, 0, 1, 0})
	// perpendicular line at x= 4, y= 9, should intersect at (4,9,0)
	p0, p1 := r3.Vector{4, 9, 22}, r3.Vector{4, 9, 12.3}
	result := plane.Intersect(p0, p1)
	test.That(t, result, test.ShouldNotBeNil)
	test.That(t, result.X, test.ShouldAlmostEqual, 4.0)
	test.That(t, result.Y, test.ShouldAlmostEqual, 9.0)
	test.That(t, result.Z, test.ShouldAlmostEqual, 0.0)
	// parallel line at z=4 should return nil
	p0, p1 = r3.Vector{4, 9, 4}, r3.Vector{22, -3, 4}
	result = plane.Intersect(p0, p1)
	test.That(t, result, test.ShouldBeNil)
	// tilted line with slope of 1 should intersect at (2, 9, 0)
	p0, p1 = r3.Vector{4, 9, 2}, r3.Vector{3, 9, 1}
	result = plane.Intersect(p0, p1)
	test.That(t, result, test.ShouldNotBeNil)
	test.That(t, result.X, test.ShouldAlmostEqual, 2.0)
	test.That(t, result.Y, test.ShouldAlmostEqual, 9.0)
	test.That(t, result.Z, test.ShouldAlmostEqual, 0.0)
	// if p1 is before p0, should still give the same result
	result = plane.Intersect(p1, p0)
	test.That(t, result, test.ShouldNotBeNil)
	test.That(t, result.X, test.ShouldAlmostEqual, 2.0)
	test.That(t, result.Y, test.ShouldAlmostEqual, 9.0)
	test.That(t, result.Z, test.ShouldAlmostEqual, 0.0)
}
