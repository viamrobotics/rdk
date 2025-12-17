package pointcloud

import (
	"errors"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func makePointCloud(t *testing.T) PointCloud {
	t.Helper()
	cloud := NewBasicPointCloud(0)
	p0 := r3.Vector{0, 0, 0}
	test.That(t, cloud.Set(p0, nil), test.ShouldBeNil)
	p1 := r3.Vector{1, 1, 1}
	test.That(t, cloud.Set(p1, nil), test.ShouldBeNil)
	p2 := r3.Vector{2, 2, 2}
	test.That(t, cloud.Set(p2, nil), test.ShouldBeNil)
	p3 := r3.Vector{3, 3, 3}
	test.That(t, cloud.Set(p3, nil), test.ShouldBeNil)
	n1 := r3.Vector{-1.1, -1.1, -1.1}
	test.That(t, cloud.Set(n1, nil), test.ShouldBeNil)
	n2 := r3.Vector{-2.2, -2.2, -2.2}
	test.That(t, cloud.Set(n2, nil), test.ShouldBeNil)
	n3 := r3.Vector{-3.2, -3.2, -3.2}
	test.That(t, cloud.Set(n3, nil), test.ShouldBeNil)
	// outlier points
	o2 := r3.Vector{2000, 2000, 2000}
	test.That(t, cloud.Set(o2, nil), test.ShouldBeNil)
	return cloud
}

func TestLessThan(t *testing.T) {
	vs := kdValues{
		treeComparableR3Vector{r3.Vector{0, 0, 0}},
		treeComparableR3Vector{r3.Vector{1, 1, 1}},
		treeComparableR3Vector{r3.Vector{-1.1, -1.1, -1.1}},
		treeComparableR3Vector{r3.Vector{2, 2, 2}},
		treeComparableR3Vector{r3.Vector{-2.2, -2.2, -2.2}},
		treeComparableR3Vector{r3.Vector{3, 3, 3}},
		treeComparableR3Vector{r3.Vector{-3.2, -3.2, -3.2}},
	}

	slicer := kdValuesSlicer{vs: vs}
	test.That(t, slicer.Less(0, 1), test.ShouldBeTrue)
	test.That(t, slicer.Less(1, 0), test.ShouldBeFalse)
	test.That(t, slicer.Less(1, 2), test.ShouldBeTrue)
	test.That(t, slicer.Less(2, 1), test.ShouldBeFalse)
	test.That(t, slicer.Less(2, 3), test.ShouldBeTrue)
	test.That(t, slicer.Less(3, 2), test.ShouldBeFalse)
	test.That(t, slicer.Less(3, 4), test.ShouldBeTrue)
	test.That(t, slicer.Less(4, 3), test.ShouldBeFalse)
	test.That(t, slicer.Less(4, 5), test.ShouldBeTrue)
	test.That(t, slicer.Less(5, 4), test.ShouldBeFalse)
	test.That(t, slicer.Less(5, 6), test.ShouldBeTrue)
	test.That(t, slicer.Less(6, 5), test.ShouldBeFalse)
}

func TestKDTreeNeighborFinding(t *testing.T) {
	clusterTolerance := 20.

	// Create a dense grid of points
	points := make([]r3.Vector, 0)
	for x := 0.0; x < 100.0; x += 5.0 {
		for y := 0.0; y < 100.0; y += 5.0 {
			for z := 0.0; z < 10.0; z += 5.0 {
				points = append(points, r3.Vector{X: x, Y: y, Z: z})
			}
		}
	}
	pc := NewBasicEmpty()
	for _, point := range points {
		pc.Set(point, NewBasicData())
	}

	kdTree := ToKDTree(pc)

	testPoints := []r3.Vector{
		{0, 0, 0},   // Corner
		{50, 50, 5}, // Center
		{95, 95, 5}, // Far corner
	}

	for i, testPoint := range testPoints {
		// Check KD-tree neighbor found count against a brute force search

		neighbors := kdTree.RadiusNearestNeighbors(testPoint, clusterTolerance, true)

		bruteForceCount := 0
		for _, p := range points {
			if testPoint.Distance(p) <= clusterTolerance {
				bruteForceCount++
			}
		}

		if len(neighbors) != bruteForceCount {
			t.Errorf("Mismatch for point %d (%.1f,%.1f,%.1f): KD-tree returned %d, expected %d",
				i, testPoint.X, testPoint.Y, testPoint.Z, len(neighbors), bruteForceCount)
		}
	}
}

func TestNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := ToKDTree(cloud)

	testPt := r3.Vector{3, 3, 3}
	_, got := cloud.At(3, 3, 3)
	test.That(t, got, test.ShouldBeTrue)

	nn, _, dist, _ := kd.NearestNeighbor(testPt)
	test.That(t, nn, test.ShouldResemble, r3.Vector{3, 3, 3})
	test.That(t, dist, test.ShouldEqual, 0)

	testPt = r3.Vector{0.5, 0, 0}
	nn, _, dist, _ = kd.NearestNeighbor(testPt)
	test.That(t, nn, test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, dist, test.ShouldEqual, 0.5)
}

func TestKNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := ToKDTree(cloud)

	testPt := r3.Vector{0, 0, 0}
	nns := kd.KNearestNeighbors(testPt, 3, true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	nns = kd.KNearestNeighbors(testPt, 3, false)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{2, 2, 2})

	nns = kd.KNearestNeighbors(testPt, 100, true)
	test.That(t, nns, test.ShouldHaveLength, 8)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	test.That(t, nns[3].P, test.ShouldResemble, r3.Vector{2, 2, 2})
	test.That(t, nns[4].P, test.ShouldResemble, r3.Vector{-2.2, -2.2, -2.2})
	test.That(t, nns[5].P, test.ShouldResemble, r3.Vector{3, 3, 3})
	test.That(t, nns[6].P, test.ShouldResemble, r3.Vector{-3.2, -3.2, -3.2})
	test.That(t, nns[7].P, test.ShouldResemble, r3.Vector{2000, 2000, 2000})
	nns = kd.KNearestNeighbors(testPt, 100, false)
	test.That(t, nns, test.ShouldHaveLength, 7)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{2, 2, 2})
	test.That(t, nns[3].P, test.ShouldResemble, r3.Vector{-2.2, -2.2, -2.2})
	test.That(t, nns[4].P, test.ShouldResemble, r3.Vector{3, 3, 3})
	test.That(t, nns[5].P, test.ShouldResemble, r3.Vector{-3.2, -3.2, -3.2})
	test.That(t, nns[6].P, test.ShouldResemble, r3.Vector{2000, 2000, 2000})

	testPt = r3.Vector{4, 4, 4}
	nns = kd.KNearestNeighbors(testPt, 2, true)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{3, 3, 3})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{2, 2, 2})
	nns = kd.KNearestNeighbors(testPt, 2, false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{3, 3, 3})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{2, 2, 2})
}

func TestRadiusNearestNeighor(t *testing.T) {
	cloud := makePointCloud(t)
	kd := ToKDTree(cloud)

	testPt := r3.Vector{0, 0, 0}
	nns := kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{1, 1, 1})

	testPt = r3.Vector{0, 0, 0}
	nns = kd.RadiusNearestNeighbors(testPt, 1.2*math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	nns = kd.RadiusNearestNeighbors(testPt, 1.2*math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{1, 1, 1})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})

	testPt = r3.Vector{-2.2, -2.2, -2.2}
	nns = kd.RadiusNearestNeighbors(testPt, 1.3*math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 3)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{-2.2, -2.2, -2.2})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{-3.2, -3.2, -3.2})
	test.That(t, nns[2].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})
	nns = kd.RadiusNearestNeighbors(testPt, 1.3*math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 2)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{-3.2, -3.2, -3.2})
	test.That(t, nns[1].P, test.ShouldResemble, r3.Vector{-1.1, -1.1, -1.1})

	testPt = r3.Vector{4, 4, 4}
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{3, 3, 3})
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 1)
	test.That(t, nns[0].P, test.ShouldResemble, r3.Vector{3, 3, 3})

	testPt = r3.Vector{5, 5, 5}
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), true)
	test.That(t, nns, test.ShouldHaveLength, 0)
	nns = kd.RadiusNearestNeighbors(testPt, math.Sqrt(3), false)
	test.That(t, nns, test.ShouldHaveLength, 0)
}

func TestNewEmptyKDtree(t *testing.T) {
	pt0 := r3.Vector{0, 0, 0}
	pt1 := r3.Vector{0, 0, 1}
	// empty tree
	pc := NewBasicPointCloud(0)
	kdt := ToKDTree(pc)
	_, _, d, got := kdt.NearestNeighbor(pt0)
	test.That(t, got, test.ShouldBeFalse)
	test.That(t, d, test.ShouldEqual, 0.)
	ps := kdt.KNearestNeighbors(pt0, 5, false)
	test.That(t, ps, test.ShouldResemble, []*PointAndData{})
	ps = kdt.RadiusNearestNeighbors(pt0, 3.2, false)
	test.That(t, ps, test.ShouldResemble, []*PointAndData{})
	// add one point
	err := kdt.Set(pt1, nil)
	test.That(t, err, test.ShouldBeNil)
	p, _, d, _ := kdt.NearestNeighbor(pt0)
	test.That(t, p, test.ShouldResemble, pt1)
	test.That(t, d, test.ShouldEqual, 1.)
	ps = kdt.KNearestNeighbors(pt0, 5, false)
	test.That(t, ps, test.ShouldHaveLength, 1)
	test.That(t, ps[0].P, test.ShouldResemble, pt1)
	ps = kdt.RadiusNearestNeighbors(pt0, 3.2, false)
	test.That(t, ps, test.ShouldHaveLength, 1)
	test.That(t, ps[0].P, test.ShouldResemble, pt1)
}

func TestStatisticalOutlierFilter(t *testing.T) {
	_, err := StatisticalOutlierFilter(-1, 2.0)
	test.That(t, err, test.ShouldBeError, errors.New("argument meanK must be a positive int, got -1"))
	_, err = StatisticalOutlierFilter(4, 0.0)
	test.That(t, err, test.ShouldBeError, errors.New("argument stdDevThresh must be a positive float, got 0.00"))

	filter, err := StatisticalOutlierFilter(3, 1.5)
	test.That(t, err, test.ShouldBeNil)
	cloud := makePointCloud(t)
	kd := ToKDTree(cloud)

	filtered := NewBasicPointCloud(0)
	err = filter(kd, filtered)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, CloudContains(filtered, 0, 0, 0), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 1, 1, 1), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -1.1, -1.1, -1.1), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 2, 2, 2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -2.2, -2.2, -2.2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 3, 3, 3), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -3.2, -3.2, -3.2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 2000, 2000, 2000), test.ShouldBeFalse)

	filtered = NewBasicPointCloud(0)
	err = filter(kd, filtered)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, CloudContains(filtered, 0, 0, 0), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 1, 1, 1), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -1.1, -1.1, -1.1), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 2, 2, 2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -2.2, -2.2, -2.2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 3, 3, 3), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, -3.2, -3.2, -3.2), test.ShouldBeTrue)
	test.That(t, CloudContains(filtered, 2000, 2000, 2000), test.ShouldBeFalse)
}
