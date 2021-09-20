package pointcloud

import (
	"math"

	"testing"

	"go.viam.com/test"
)

func TestEmptyPlane(t *testing.T) {
	plane := NewEmptyPlane()
	test.That(t, plane.Equation(), test.ShouldResemble, [4]float64{})
	test.That(t, plane.Normal(), test.ShouldResemble, Vec3{})
	test.That(t, plane.Center(), test.ShouldResemble, Vec3{})
	test.That(t, plane.Offset(), test.ShouldEqual, 0.0)
	cloud, err := plane.PointCloud()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloud, test.ShouldNotBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 0)
	pt := Vec3{1, 2, 3}
	test.That(t, plane.Distance(pt), test.ShouldEqual, 0)
}

func TestNewPlane(t *testing.T) {
	// make the point cloud, a diamond of slope 1 in x and y
	pc := New()
	p0 := NewBasicPoint(0., 0., 0.)
	test.That(t, pc.Set(p0), test.ShouldBeNil)
	p1 := NewBasicPoint(0., 2., 2.)
	test.That(t, pc.Set(p1), test.ShouldBeNil)
	p2 := NewBasicPoint(2., 0., 2.)
	test.That(t, pc.Set(p2), test.ShouldBeNil)
	p3 := NewBasicPoint(2., 2., 4.)
	test.That(t, pc.Set(p3), test.ShouldBeNil)
	eq := [4]float64{1, 1, -1, 0}
	// run the tests
	plane := NewPlane(pc, eq)
	test.That(t, plane.Equation(), test.ShouldResemble, eq)
	test.That(t, plane.Normal(), test.ShouldResemble, Vec3{1, 1, -1})
	test.That(t, plane.Center(), test.ShouldResemble, Vec3{1, 1, 2})
	test.That(t, plane.Offset(), test.ShouldEqual, 0.0)
	cloud, err := plane.PointCloud()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloud, test.ShouldNotBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 4)
	pt := Vec3{-1, -1, 1}
	test.That(t, math.Abs(plane.Distance(pt)), test.ShouldAlmostEqual, math.Sqrt(3))
}

func TestPointPosition(t *testing.T) {
	p0 := NewBasicPoint(0., 0., 0.)
	p1 := NewBasicPoint(0., 2., 2.)
	p2 := NewBasicPoint(2., 0., 2.)
	p3 := NewBasicPoint(2., 2., 4.)
	points := []Point{p0, p1, p2, p3}
	positions := GetPositions(points)
	test.That(t, Vec3(positions[0]), test.ShouldResemble, Vec3{0, 0, 0})
	test.That(t, Vec3(positions[1]), test.ShouldResemble, Vec3{0, 2, 2})
	test.That(t, Vec3(positions[2]), test.ShouldResemble, Vec3{2, 0, 2})
	test.That(t, Vec3(positions[3]), test.ShouldResemble, Vec3{2, 2, 4})
}
