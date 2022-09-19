package vision

import (
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
	expectedBox, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{0.5, 0.5, 0}), r3.Vector{1, 1, 0})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, obj.Geometry.AlmostEqual(expectedBox), test.ShouldBeTrue)
}
