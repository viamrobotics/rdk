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
	cloudMap := make(map[pc.Vec3]int)
	clouds := make([]pc.PointCloud, 0)
	for i := 0; i < 3; i++ {
		clouds = append(clouds, pc.New())
	}

	// create 1st cloud
	p00 := pc.NewBasicPoint(0, 0, 0)
	cloudMap[p00.Position()] = 0
	test.That(t, clouds[0].Set(p00), test.ShouldBeNil)
	p01 := pc.NewBasicPoint(0, 0, 1)
	cloudMap[p01.Position()] = 0
	test.That(t, clouds[0].Set(p01), test.ShouldBeNil)
	p02 := pc.NewBasicPoint(0, 1, 0)
	cloudMap[p02.Position()] = 0
	test.That(t, clouds[0].Set(p02), test.ShouldBeNil)
	p03 := pc.NewBasicPoint(0, 1, 1)
	cloudMap[p03.Position()] = 0
	test.That(t, clouds[0].Set(p03), test.ShouldBeNil)
	testPointCloudBoundingBox(t, clouds[0], r3.Vector{0, 0.5, 0.5}, r3.Vector{0, 1, 1})
	obj, err := vision.NewObject(clouds[0])
	test.That(t, err, test.ShouldBeNil)
	clusters = append(clusters, obj)

	// create a 2nd cloud far away
	p10 := pc.NewBasicPoint(30, 0, 0)
	cloudMap[p10.Position()] = 1
	test.That(t, clouds[1].Set(p10), test.ShouldBeNil)
	p11 := pc.NewBasicPoint(30, 0, 1)
	cloudMap[p11.Position()] = 1
	test.That(t, clouds[1].Set(p11), test.ShouldBeNil)
	p12 := pc.NewBasicPoint(30, 1, 0)
	cloudMap[p12.Position()] = 1
	test.That(t, clouds[1].Set(p12), test.ShouldBeNil)
	p13 := pc.NewBasicPoint(30, 1, 1)
	cloudMap[p13.Position()] = 1
	test.That(t, clouds[1].Set(p13), test.ShouldBeNil)
	testPointCloudBoundingBox(t, clouds[1], r3.Vector{30, 0.5, 0.5}, r3.Vector{0, 1, 1})
	obj, err = vision.NewObject(clouds[1])
	test.That(t, err, test.ShouldBeNil)
	clusters = append(clusters, obj)

	// create 3rd cloud
	p20 := pc.NewBasicPoint(0, 30, 0)
	cloudMap[p20.Position()] = 2
	test.That(t, clouds[2].Set(p20), test.ShouldBeNil)
	p21 := pc.NewBasicPoint(0, 30, 1)
	cloudMap[p21.Position()] = 2
	test.That(t, clouds[2].Set(p21), test.ShouldBeNil)
	p22 := pc.NewBasicPoint(1, 30, 0)
	cloudMap[p22.Position()] = 2
	test.That(t, clouds[2].Set(p22), test.ShouldBeNil)
	p23 := pc.NewBasicPoint(1, 30, 1)
	cloudMap[p23.Position()] = 2
	test.That(t, clouds[2].Set(p23), test.ShouldBeNil)
	p24 := pc.NewBasicPoint(0.5, 30, 0.5)
	cloudMap[p24.Position()] = 2
	test.That(t, clouds[2].Set(p24), test.ShouldBeNil)
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
	p30 := pc.NewBasicPoint(30, 30, 1)
	test.That(t, clusters.AssignCluster(p30, 3), test.ShouldBeNil)
	test.That(t, clusters.N(), test.ShouldEqual, 4)
	test.That(t, clusters.Indices[p30.Position()], test.ShouldEqual, 3)
	testPointCloudBoundingBox(t, clusters.Objects[3], r3.Vector{30, 30, 1}, r3.Vector{})

	// assign a new cluster with a large index
	pNew := pc.NewBasicPoint(30, 30, 30)
	test.That(t, clusters.AssignCluster(pNew, 100), test.ShouldBeNil)
	test.That(t, clusters.N(), test.ShouldEqual, 101)
	test.That(t, clusters.Indices[pNew.Position()], test.ShouldEqual, 100)
	testPointCloudBoundingBox(t, clusters.Objects[100], r3.Vector{30, 30, 30}, r3.Vector{})
}

func TestMergeCluster(t *testing.T) {
	clusters := createPointClouds(t)

	// before merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 4)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 4)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 5)
	for i := 0; i < 2; i++ {
		clusters.Objects[i].Iterate(func(pt pc.Point) bool {
			test.That(t, clusters.Indices[pt.Position()], test.ShouldEqual, i)
			return true
		})
	}

	// merge
	test.That(t, clusters.MergeClusters(0, 1), test.ShouldBeNil)

	// after merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 8)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 5)
	clusters.Objects[1].Iterate(func(pt pc.Point) bool {
		test.That(t, clusters.Indices[pt.Position()], test.ShouldEqual, 1)
		return true
	})
	test.That(t, clusters.Objects[0].BoundingBox, test.ShouldBeNil)
	testPointCloudBoundingBox(t, clusters.Objects[1].PointCloud, r3.Vector{15, 0.5, 0.5}, r3.Vector{30, 1, 1})
	testPointCloudBoundingBox(t, clusters.Objects[2].PointCloud, r3.Vector{0.5, 30, 0.5}, r3.Vector{1, 0, 1})

	// merge to new cluster
	test.That(t, clusters.MergeClusters(2, 3), test.ShouldBeNil)

	// after merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 8)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[3].Size(), test.ShouldEqual, 5)
	test.That(t, clusters.Objects[0].BoundingBox, test.ShouldBeNil)
	testPointCloudBoundingBox(t, clusters.Objects[1].PointCloud, r3.Vector{15, 0.5, 0.5}, r3.Vector{30, 1, 1})
	test.That(t, clusters.Objects[2].BoundingBox, test.ShouldBeNil)
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
		boxExpected, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(center), dims)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(boxExpected), test.ShouldBeTrue)
	}
}
