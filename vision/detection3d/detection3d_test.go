package detection3d_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/detection3d"
)

func TestFromSegmenter(t *testing.T) {
	cloud := pointcloud.NewBasicEmpty()
	test.That(t, cloud.Set(r3.Vector{X: 10}, pointcloud.NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(r3.Vector{X: 12}, pointcloud.NewBasicData()), test.ShouldBeNil)
	mug, err := vision.NewObjectWithLabel(cloud, "mug", nil)
	test.That(t, err, test.ShouldBeNil)
	segmenter := func(ctx context.Context, src camera.Camera) ([]*vision.Object, error) {
		return []*vision.Object{mug, vision.NewEmptyObject()}, nil
	}

	dets, err := detection3d.FromSegmenter("seg", segmenter)(context.Background(), inject.NewCamera("cam"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(dets), test.ShouldEqual, 2)

	t.Run("object becomes a recentered root plus a points child", func(t *testing.T) {
		test.That(t, len(dets[0].Transforms), test.ShouldEqual, 2)
		root, points := dets[0].Transforms[0], dets[0].Transforms[1]
		test.That(t, root.Name(), test.ShouldEqual, "seg/object-0")
		test.That(t, root.Parent(), test.ShouldEqual, "cam")
		test.That(t, spatialmath.R3VectorAlmostEqual(root.Pose().Point(), r3.Vector{X: 11}, 1e-6), test.ShouldBeTrue)
		test.That(t, spatialmath.PoseAlmostEqual(root.Geometry().Pose(), spatialmath.NewZeroPose()), test.ShouldBeTrue)
		test.That(t, root.Geometry().Label(), test.ShouldEqual, "mug")

		test.That(t, points.Name(), test.ShouldEqual, "seg/object-0/points")
		test.That(t, points.Parent(), test.ShouldEqual, "seg/object-0")
		octree, ok := points.Geometry().(*pointcloud.BasicOctree)
		test.That(t, ok, test.ShouldBeTrue)
		var xs []float64
		octree.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			xs = append(xs, p.X)
			return true
		})
		test.That(t, xs, test.ShouldHaveLength, 2)
		test.That(t, xs, test.ShouldContain, -1.0)
		test.That(t, xs, test.ShouldContain, 1.0)
	})

	t.Run("empty object has no geometry and no points child", func(t *testing.T) {
		test.That(t, len(dets[1].Transforms), test.ShouldEqual, 1)
		test.That(t, dets[1].Transforms[0].Geometry(), test.ShouldBeNil)
	})
}
