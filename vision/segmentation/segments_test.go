package segmentation

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/vision"
)

func createPointClouds(t *testing.T) *Segments {
	t.Helper()
	clusters := make([]*vision.Object, 0)
	cloudMap := make(map[r3.Vector]int)
	clouds := make([]pc.PointCloud, 0)
	for i := 0; i < 3; i++ {
		clouds = append(clouds, pc.New())
	}

	// create 1st cloud
	p00 := pc.NewVector(0, 0, 0)
	cloudMap[p00] = 0
	test.That(t, clouds[0].Set(p00, nil), test.ShouldBeNil)
	p01 := pc.NewVector(0, 0, 1)
	cloudMap[p01] = 0
	test.That(t, clouds[0].Set(p01, nil), test.ShouldBeNil)
	p02 := pc.NewVector(0, 1, 0)
	cloudMap[p02] = 0
	test.That(t, clouds[0].Set(p02, nil), test.ShouldBeNil)
	p03 := pc.NewVector(0, 1, 1)
	cloudMap[p03] = 0
	test.That(t, clouds[0].Set(p03, nil), test.ShouldBeNil)
	testPointCloudBoundingBox(t, clouds[0], r3.Vector{0, 0.5, 0.5}, r3.Vector{0, 1, 1})
	obj, err := vision.NewObject(clouds[0])
	test.That(t, err, test.ShouldBeNil)
	clusters = append(clusters, obj)

	// create a 2nd cloud far away
	p10 := pc.NewVector(30, 0, 0)
	cloudMap[p10] = 1
	test.That(t, clouds[1].Set(p10, nil), test.ShouldBeNil)
	p11 := pc.NewVector(30, 0, 1)
	cloudMap[p11] = 1
	test.That(t, clouds[1].Set(p11, nil), test.ShouldBeNil)
	p12 := pc.NewVector(30, 1, 0)
	cloudMap[p12] = 1
	test.That(t, clouds[1].Set(p12, nil), test.ShouldBeNil)
	p13 := pc.NewVector(30, 1, 1)
	cloudMap[p13] = 1
	test.That(t, clouds[1].Set(p13, nil), test.ShouldBeNil)
	testPointCloudBoundingBox(t, clouds[1], r3.Vector{30, 0.5, 0.5}, r3.Vector{0, 1, 1})
	obj, err = vision.NewObject(clouds[1])
	test.That(t, err, test.ShouldBeNil)
	clusters = append(clusters, obj)

	// create 3rd cloud
	p20 := pc.NewVector(0, 30, 0)
	cloudMap[p20] = 2
	test.That(t, clouds[2].Set(p20, nil), test.ShouldBeNil)
	p21 := pc.NewVector(0, 30, 1)
	cloudMap[p21] = 2
	test.That(t, clouds[2].Set(p21, nil), test.ShouldBeNil)
	p22 := pc.NewVector(1, 30, 0)
	cloudMap[p22] = 2
	test.That(t, clouds[2].Set(p22, nil), test.ShouldBeNil)
	p23 := pc.NewVector(1, 30, 1)
	cloudMap[p23] = 2
	test.That(t, clouds[2].Set(p23, nil), test.ShouldBeNil)
	p24 := pc.NewVector(0.5, 30, 0.5)
	cloudMap[p24] = 2
	test.That(t, clouds[2].Set(p24, nil), test.ShouldBeNil)
	testPointCloudBoundingBox(t, clouds[2], r3.Vector{0.5, 30, 0.5}, r3.Vector{1, 0, 1})
	obj, err = vision.NewObject(clouds[2])
	test.That(t, err, test.ShouldBeNil)
	clusters = append(clusters, obj)
	return &Segments{clusters, cloudMap}
}

func TestAssignCluter(t *testing.T) {
	clusters := createPointClouds(t)
	test.That(t, clusters.N(), test.ShouldEqual, 3)

	// assign a new cluster
	p30 := pc.NewVector(30, 30, 1)
	test.That(t, clusters.AssignCluster(p30, nil, 3), test.ShouldBeNil)
	test.That(t, clusters.N(), test.ShouldEqual, 4)
	test.That(t, clusters.Indices[p30], test.ShouldEqual, 3)
	testPointCloudBoundingBox(t, clusters.Objects[3], r3.Vector{30, 30, 1}, r3.Vector{})

	// assign a new cluster with a large index
	pNew := pc.NewVector(30, 30, 30)
	test.That(t, clusters.AssignCluster(pNew, nil, 100), test.ShouldBeNil)
	test.That(t, clusters.N(), test.ShouldEqual, 101)
	test.That(t, clusters.Indices[pNew], test.ShouldEqual, 100)
	testPointCloudBoundingBox(t, clusters.Objects[100], r3.Vector{30, 30, 30}, r3.Vector{})
}

func TestMergeCluster(t *testing.T) {
	clusters := createPointClouds(t)

	// before merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 4)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 4)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 5)
	for i := 0; i < 2; i++ {
		clusters.Objects[i].Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
			test.That(t, clusters.Indices[pt], test.ShouldEqual, i)
			return true
		})
	}

	// merge
	test.That(t, clusters.MergeClusters(0, 1), test.ShouldBeNil)

	// after merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 8)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 5)
	clusters.Objects[1].Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		test.That(t, clusters.Indices[pt], test.ShouldEqual, 1)
		return true
	})
	test.That(t, clusters.Objects[0].Geometry, test.ShouldBeNil)
	testPointCloudBoundingBox(t, clusters.Objects[1].PointCloud, r3.Vector{15, 0.5, 0.5}, r3.Vector{30, 1, 1})
	testPointCloudBoundingBox(t, clusters.Objects[2].PointCloud, r3.Vector{0.5, 30, 0.5}, r3.Vector{1, 0, 1})

	// merge to new cluster
	test.That(t, clusters.MergeClusters(2, 3), test.ShouldBeNil)

	// after merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 8)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[3].Size(), test.ShouldEqual, 5)
	test.That(t, clusters.Objects[0].Geometry, test.ShouldBeNil)
	testPointCloudBoundingBox(t, clusters.Objects[1].PointCloud, r3.Vector{15, 0.5, 0.5}, r3.Vector{30, 1, 1})
	test.That(t, clusters.Objects[2].Geometry, test.ShouldBeNil)
	testPointCloudBoundingBox(t, clusters.Objects[3].PointCloud, r3.Vector{0.5, 30, 0.5}, r3.Vector{1, 0, 1})
}

func testPointCloudBoundingBox(t *testing.T, cloud pc.PointCloud, center, dims r3.Vector) {
	t.Helper()
	box, err := pc.BoundingBoxFromPointCloud(cloud)
	if cloud.Size() == 0 {
		test.That(t, box, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
	} else {
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)
		boxExpected, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(center), dims, "")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(boxExpected), test.ShouldBeTrue)
	}
}
