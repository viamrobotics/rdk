package segmentation

import (
	"testing"

	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
)

func createPointClouds(t *testing.T) *Segments {
	t.Helper()
	clusters := make([]*PointCloudWithMeta, 0)
	cloudMap := make(map[pc.Vec3]int)
	clouds := make([]pc.PointCloud, 0)
	means := make([]pc.Vec3, 0)
	boxes := make([]pc.BoxGeometry, 0)
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
	means = append(means, pc.Vec3{0, 0.5, 0.5})
	boxes = append(boxes, pc.BoxGeometry{0, 1, 1})
	test.That(t, pc.CalculateMeanOfPointCloud(clouds[0]), test.ShouldResemble, means[0])
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clouds[0]), test.ShouldResemble, boxes[0])
	clusters = append(clusters, NewPointCloudWithMeta(clouds[0]))
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
	means = append(means, pc.Vec3{30, 0.5, 0.5})
	boxes = append(boxes, pc.BoxGeometry{0, 1, 1})
	test.That(t, pc.CalculateMeanOfPointCloud(clouds[1]), test.ShouldResemble, means[1])
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clouds[1]), test.ShouldResemble, boxes[1])
	clusters = append(clusters, NewPointCloudWithMeta(clouds[1]))
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
	means = append(means, pc.Vec3{0.5, 30, 0.5})
	boxes = append(boxes, pc.BoxGeometry{1, 0, 1})
	test.That(t, pc.CalculateMeanOfPointCloud(clouds[2]), test.ShouldResemble, means[2])
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clouds[2]), test.ShouldResemble, boxes[2])
	clusters = append(clusters, NewPointCloudWithMeta(clouds[2]))
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
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[3]), test.ShouldResemble, pc.Vec3{30, 30, 1})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[3]), test.ShouldResemble, pc.BoxGeometry{0, 0, 0})
	// assign a new cluster with a large index
	pNew := pc.NewBasicPoint(30, 30, 30)
	test.That(t, clusters.AssignCluster(pNew, 100), test.ShouldBeNil)
	test.That(t, clusters.N(), test.ShouldEqual, 101)
	test.That(t, clusters.Indices[pNew.Position()], test.ShouldEqual, 100)
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[100]), test.ShouldResemble, pc.Vec3{30, 30, 30})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[100]), test.ShouldResemble, pc.BoxGeometry{0, 0, 0})
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
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[0]), test.ShouldResemble, pc.Vec3{})
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[1]), test.ShouldResemble, pc.Vec3{15, 0.5, 0.5})
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[2]), test.ShouldResemble, pc.Vec3{0.5, 30, 0.5})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[0]), test.ShouldResemble, pc.BoxGeometry{})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[1]), test.ShouldResemble, pc.BoxGeometry{30, 1, 1})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[2]), test.ShouldResemble, pc.BoxGeometry{1, 0, 1})

	// merge to new cluster
	test.That(t, clusters.MergeClusters(2, 3), test.ShouldBeNil)
	// after merge
	test.That(t, clusters.Objects[0].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[1].Size(), test.ShouldEqual, 8)
	test.That(t, clusters.Objects[2].Size(), test.ShouldEqual, 0)
	test.That(t, clusters.Objects[3].Size(), test.ShouldEqual, 5)
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[0]), test.ShouldResemble, pc.Vec3{})
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[1]), test.ShouldResemble, pc.Vec3{15, 0.5, 0.5})
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[2]), test.ShouldResemble, pc.Vec3{})
	test.That(t, pc.CalculateMeanOfPointCloud(clusters.Objects[3]), test.ShouldResemble, pc.Vec3{0.5, 30, 0.5})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[0]), test.ShouldResemble, pc.BoxGeometry{})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[1]), test.ShouldResemble, pc.BoxGeometry{30, 1, 1})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[2]), test.ShouldResemble, pc.BoxGeometry{})
	test.That(t, pc.CalculateBoundingBoxOfPointCloud(clusters.Objects[3]), test.ShouldResemble, pc.BoxGeometry{1, 0, 1})
}
