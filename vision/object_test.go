package vision

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

func TestObjectCreation(t *testing.T) {
	// test empty objects
	obj, err := NewObject(nil)
	test.That(t, err, test.ShouldBeNil)
	obj2 := NewEmptyObject()
	test.That(t, obj, test.ShouldResemble, obj2)

	// create from point cloud
	pc := pointcloud.New()
	err = pc.Set(pointcloud.NewVector(0, 0, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewVector(0, 1, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewVector(1, 0, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewVector(1, 1, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	obj, err = NewObject(pc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, obj.PointCloud, test.ShouldResemble, pc)
	expectedBox, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0.5, 0.5, 0}), r3.Vector{1, 1, 0}, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, obj.Geometry.AlmostEqual(expectedBox), test.ShouldBeTrue)
}

func TestObjectDistance(t *testing.T) {
	pc := pointcloud.New()
	err := pc.Set(pointcloud.NewVector(0, 0, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	obj, err := NewObject(pc)
	test.That(t, err, test.ShouldBeNil)
	dist, err := obj.Distance()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldEqual, 0)

	err = pc.Set(pointcloud.NewVector(0, 1, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewVector(1, 0, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewVector(1, 1, 0), nil)
	test.That(t, err, test.ShouldBeNil)
	obj, err = NewObject(pc)
	test.That(t, err, test.ShouldBeNil)
	dist, err = obj.Distance()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldEqual, math.Sqrt(math.Pow(0.5, 2)+math.Pow(0.5, 2)))

	err = pc.Set(pointcloud.NewVector(0, 0, -3), nil)
	test.That(t, err, test.ShouldBeNil)
	obj, err = NewObject(pc)
	test.That(t, err, test.ShouldBeNil)
	dist, err = obj.Distance()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dist, test.ShouldEqual, math.Sqrt(math.Pow(0.4, 2)+math.Pow(0.4, 2)+math.Pow(0.6, 2)))

	obj = NewEmptyObject()
	_, err = obj.Distance()
	test.That(t, err.Error(), test.ShouldContainSubstring, "no geometry object")
}
