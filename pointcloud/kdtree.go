package pointcloud

import "gonum.org/v1/gonum/spatial/kdtree"

// KDTree extends PointCloud and orders the points in 3D space to implement nearest neighbor algos.
type KDTree struct {
	PointCloud
	tree    *kdtree.Tree
	rebuild bool
}

// NewKDTree creates a KDTree from an input PointCloud.
func NewKDTree(pc PointCloud) *KDTree {
	if pc.Size() == 0 {
		return nil
	}
	if k, ok := pc.(*KDTree); ok { // rebuild the KDTree from scratch
		pc = k.PointCloud
	}
	points := Points(pc.Points())
	tree := kdtree.New(points, false)

	return &KDTree{pc, tree, false}
}

// Set adds a new point to the PointCloud and tree. Does not rebalance the tree.
func (kd *KDTree) Set(p Point) error {
	kd.tree.Insert(p, false)
	return kd.PointCloud.Set(p)
}

// Unset removes the point from the PointCloud and sets a flag to rebuild the tree next time NN is used.
func (kd *KDTree) Unset(x, y, z float64) {
	kd.PointCloud.Unset(x, y, z)
	kd.rebuild = true
}

// NearestNeighbor returns the nearest point and its distance from the input point.
func (kd *KDTree) NearestNeighbor(p Point) (Point, float64) {
	if kd.rebuild {
		points := Points(kd.Points())
		kd.tree = kdtree.New(points, false)
		kd.rebuild = false
	}
	c, dist := kd.tree.Nearest(p)
	p2, ok := c.(Point)
	if !ok {
		panic("kdtree.Comparable is not a Point")
	}
	return p2, dist
}

// KNearestNeighbors returns the k nearest points ordered by distance. if includeSelf is true and if the point p
// is in the point cloud, point p will also be returned in the slice as the first element with distance 0.
func (kd *KDTree) KNearestNeighbors(p Point, k int, includeSelf bool) []Point {
	if kd.rebuild {
		points := Points(kd.Points())
		kd.tree = kdtree.New(points, false)
		kd.rebuild = false
	}
	start := 0
	if kd.At(p.Position().X, p.Position().Y, p.Position().Z) != nil && !includeSelf {
		k++
		start++
	}
	keep := kdtree.NewNKeeper(k)
	kd.tree.NearestSet(keep, p)
	nearestPoints := make([]Point, 0, keep.Heap.Len())
	for i := start; i < keep.Heap.Len(); i++ {
		c := keep.Heap[i]
		p, ok := c.Comparable.(Point)
		if !ok {
			panic("kdtree.Comparable is not a Point")
		}
		nearestPoints = append(nearestPoints, p)
	}
	return nearestPoints
}

// RadiusNearestNeighbors returns the nearest points within a radius r (inclusive) ordered by distance.
// If includeSelf is true and if the point p is in the point cloud, point p will also be returned in the slice
// as the first element with distance 0.
func (kd *KDTree) RadiusNearestNeighbors(p Point, r float64, includeSelf bool) []Point {
	if kd.rebuild {
		points := Points(kd.Points())
		kd.tree = kdtree.New(points, false)
		kd.rebuild = false
	}
	start := 0
	if kd.At(p.Position().X, p.Position().Y, p.Position().Z) != nil && !includeSelf {
		start++
	}
	keep := kdtree.NewDistKeeper(r)
	kd.tree.NearestSet(keep, p)
	nearestPoints := make([]Point, 0, keep.Heap.Len())
	for i := start; i < keep.Heap.Len(); i++ {
		c := keep.Heap[i]
		p, ok := c.Comparable.(Point)
		if !ok {
			panic("kdtree.Comparable is not a Point")
		}
		nearestPoints = append(nearestPoints, p)
	}
	return nearestPoints
}

// Points is a slice type that satisfies kdtree.Interface.
type Points []Point

// Index returns the point at index i.
func (ps Points) Index(i int) kdtree.Comparable { return ps[i] }

// Len returns the length of the slice.
func (ps Points) Len() int { return len(ps) }

// Slice returns the subset of the slice.
func (ps Points) Slice(start, end int) kdtree.Interface { return ps[start:end] }

// Pivot chooses the median point along an axis to be the pivot element.
func (ps Points) Pivot(d kdtree.Dim) int {
	return pointsHelper{Points: ps, Dim: d}.Pivot()
}

// pointsHelper is required to help Points.
type pointsHelper struct {
	kdtree.Dim
	Points
}

func (ph pointsHelper) Less(i, j int) bool {
	switch ph.Dim {
	case 0:
		return ph.Points[i].Position().X < ph.Points[j].Position().X
	case 1:
		return ph.Points[i].Position().Y < ph.Points[j].Position().Y
	case 2:
		return ph.Points[i].Position().Z < ph.Points[j].Position().Z
	default:
		panic("illegal dimension")
	}
}

func (ph pointsHelper) Pivot() int {
	return kdtree.Partition(ph, kdtree.MedianOfMedians(ph))
}

func (ph pointsHelper) Slice(start, end int) kdtree.SortSlicer {
	ph.Points = ph.Points[start:end]
	return ph
}

func (ph pointsHelper) Swap(i, j int) {
	ph.Points[i], ph.Points[j] = ph.Points[j], ph.Points[i]
}
