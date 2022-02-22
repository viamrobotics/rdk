package vision

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
)

func TestObjectCreation(t *testing.T) {
	// test empty objects
	obj := NewObject(nil)
	obj2 := NewEmptyObject()
	test.That(t, obj, test.ShouldResemble, obj2)

	// create from point cloud
	pc := pointcloud.New()
	err := pc.Set(pointcloud.NewBasicPoint(0, 0, 0))
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewBasicPoint(0, 1, 0))
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewBasicPoint(1, 0, 0))
	test.That(t, err, test.ShouldBeNil)
	err = pc.Set(pointcloud.NewBasicPoint(1, 1, 0))
	test.That(t, err, test.ShouldBeNil)
	obj = NewObject(pc)
	test.That(t, obj.PointCloud, test.ShouldResemble, pc)
	test.That(t, obj.Center, test.ShouldResemble, pointcloud.Vec3{0.5, 0.5, 0})
	test.That(t, obj.BoundingBox, test.ShouldResemble, pointcloud.RectangularPrism{1, 1, 0})
}
