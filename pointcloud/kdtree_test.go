package pointcloud

import (
	"errors"
	"math"
	"testing"

	"go.viam.com/test"
)

func makePointCloud(t *testing.T) PointCloud {
	t.Helper()
	cloud := New()
	p0 := NewBasicPoint(0, 0, 0)
	test.That(t, cloud.Set(p0), test.ShouldBeNil)
	p1 := NewBasicPoint(1, 1, 1)
	test.That(t, cloud.Set(p1), test.ShouldBeNil)
	p2 := NewBasicPoint(2, 2, 2)
	test.That(t, cloud.Set(p2), test.ShouldBeNil)
	p3 := NewBasicPoint(3, 3, 3)
	test.That(t, cloud.Set(p3), test.ShouldBeNil)
	n1 := NewBasicPoint(-1.1, -1.1, -1.1)
	test.That(t, cloud.Set(n1), test.ShouldBeNil)
	n2 := NewBasicPoint(-2.2, -2.2, -2.2)
	test.That(t, cloud.Set(n2), test.ShouldBeNil)
	n3 := NewBasicPoint(-3.2, -3.2, -3.2)
	test.That(t, cloud.Set(n3), test.ShouldBeNil)
	// outlier points
	o2 := NewBasicPoint(2000, 2000, 2000)
	test.That(t, cloud.Set(o2), test.ShouldBeNil)
	return cloud
}

func TestNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := NewKDTree(cloud)

	testPt := cloud.At(3, 3, 3)
	nn, dist := kd.NearestNeighbor(testPt)
	test.That(t, nn, test.ShouldResemble, cloud.At(3, 3, 3))
	test.That(t, dist, test.ShouldEqual, 0)

	testPt = NewBasicPoint(0.5, 0, 0)
	nn, dist = kd.NearestNeighbor(testPt)
	test.That(t, nn, test.ShouldResemble, cloud.At(0, 0, 0))
	test.That(t, dist, test.ShouldEqual, 0.5)
}

func TestKNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := NewKDTree(cloud)

	testPt := cloud.At(0, 0, 0)
	nns := kd.KNearestNeighbors(testPt, 3, true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(0, 0, 0))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	nns = kd.KNearestNeighbors(testPt, 3, false)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(2, 2, 2))

	nns = kd.KNearestNeighbors(testPt, 100, true)
	test.That(t, nns, test.ShouldHaveLength, 8)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(0, 0, 0))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	test.That(t, nns[3], test.ShouldResemble, cloud.At(2, 2, 2))
	test.That(t, nns[4], test.ShouldResemble, cloud.At(-2.2, -2.2, -2.2))
	test.That(t, nns[5], test.ShouldResemble, cloud.At(3, 3, 3))
	test.That(t, nns[6], test.ShouldResemble, cloud.At(-3.2, -3.2, -3.2))
	test.That(t, nns[7], test.ShouldResemble, cloud.At(2000, 2000, 2000))
	nns = kd.KNearestNeighbors(testPt, 100, false)
	test.That(t, nns, test.ShouldHaveLength, 7)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(2, 2, 2))
	test.That(t, nns[3], test.ShouldResemble, cloud.At(-2.2, -2.2, -2.2))
	test.That(t, nns[4], test.ShouldResemble, cloud.At(3, 3, 3))
	test.That(t, nns[5], test.ShouldResemble, cloud.At(-3.2, -3.2, -3.2))
	test.That(t, nns[6], test.ShouldResemble, cloud.At(2000, 2000, 2000))

	testPt = NewBasicPoint(4, 4, 4)
	nns = kd.KNearestNeighbors(testPt, 2, true)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(3, 3, 3))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(2, 2, 2))
	nns = kd.KNearestNeighbors(testPt, 2, false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(3, 3, 3))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(2, 2, 2))
}

func TestRadiusNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := NewKDTree(cloud)

	testPt := cloud.At(0, 0, 0)
	nns := kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(0, 0, 0))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(1, 1, 1))
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(1, 1, 1))

	testPt = cloud.At(0, 0, 0)
	nns = kd.RadiusNearestNeighbors(testPt, 1.2*math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(0, 0, 0))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	nns = kd.RadiusNearestNeighbors(testPt, 1.2*math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(1, 1, 1))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))

	testPt = cloud.At(-2.2, -2.2, -2.2)
	nns = kd.RadiusNearestNeighbors(testPt, 1.3*math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(-2.2, -2.2, -2.2))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(-3.2, -3.2, -3.2))
	test.That(t, nns[2], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))
	nns = kd.RadiusNearestNeighbors(testPt, 1.3*math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(-3.2, -3.2, -3.2))
	test.That(t, nns[1], test.ShouldResemble, cloud.At(-1.1, -1.1, -1.1))

	testPt = NewBasicPoint(4, 4, 4)
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(3, 3, 3))
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0], test.ShouldResemble, cloud.At(3, 3, 3))

	testPt = NewBasicPoint(5, 5, 5)
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 0)
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 0)
}

func TestStatisticalOutlierFilter(t *testing.T) {
	_, err := StatisticalOutlierFilter(-1, 2.0)
	test.That(t, err, test.ShouldBeError, errors.New("argument meanK must be a positive int, got -1"))
	_, err = StatisticalOutlierFilter(4, 0.0)
	test.That(t, err, test.ShouldBeError, errors.New("argument stdDevThresh must be a positive float, got 0.00"))

	filter, err := StatisticalOutlierFilter(3, 1.5)
	test.That(t, err, test.ShouldBeNil)
	cloud := makePointCloud(t)
	kd := NewKDTree(cloud)

	filtered, err := filter(kd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, filtered.At(0, 0, 0), test.ShouldNotBeNil)
	test.That(t, filtered.At(1, 1, 1), test.ShouldNotBeNil)
	test.That(t, filtered.At(-1.1, -1.1, -1.1), test.ShouldNotBeNil)
	test.That(t, filtered.At(2, 2, 2), test.ShouldNotBeNil)
	test.That(t, filtered.At(-2.2, -2.2, -2.2), test.ShouldNotBeNil)
	test.That(t, filtered.At(3, 3, 3), test.ShouldNotBeNil)
	test.That(t, filtered.At(-3.2, -3.2, -3.2), test.ShouldNotBeNil)
	test.That(t, filtered.At(2000, 2000, 2000), test.ShouldBeNil)

	filtered, err = filter(cloud)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, filtered.At(0, 0, 0), test.ShouldNotBeNil)
	test.That(t, filtered.At(1, 1, 1), test.ShouldNotBeNil)
	test.That(t, filtered.At(-1.1, -1.1, -1.1), test.ShouldNotBeNil)
	test.That(t, filtered.At(2, 2, 2), test.ShouldNotBeNil)
	test.That(t, filtered.At(-2.2, -2.2, -2.2), test.ShouldNotBeNil)
	test.That(t, filtered.At(3, 3, 3), test.ShouldNotBeNil)
	test.That(t, filtered.At(-3.2, -3.2, -3.2), test.ShouldNotBeNil)
	test.That(t, filtered.At(2000, 2000, 2000), test.ShouldBeNil)
}
