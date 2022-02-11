package pointcloud

import "gonum.org/v1/gonum/spatial/kdtree"

type KDTree struct {
	tree *kdtree.Tree
}

func NewKDTree(pc PointCloud) *KDTree {
	if pc.Size() == 0 {
		return nil
	}
	points := Points(pc.Points())
	tree := kdtree.New(points, false)

	return &KDTree{tree}
}

func (kd *KDTree) KNearestNeighbors(p Point, k int) []Point {
	keep := kdtree.NewNKeeper(k)
	kd.tree.NearestSet(keep, p)
	nearestPoints := make([]Point, keep.Heap.Len())
	for i, c := range keep.Heap {
		p, ok := c.Comparable.(Point)
		if !ok {
			panic("kdtree.Comparable is not a Point")
		}
		nearestPoints[i] = p
	}
	return nearestPoints
}

func (kd *KDTree) RadiusNearestNeighbors(p Point, r float64) []Point {
	keep := kdtree.NewDistKeeper(r)
	kd.tree.NearestSet(keep, p)
	nearestPoints := make([]Point, keep.Heap.Len())
	for i, c := range keep.Heap {
		p, ok := c.Comparable.(Point)
		if !ok {
			panic("kdtree.Comparable is not a Point")
		}
		nearestPoints[i] = p
	}
	return nearestPoints
}

// Points is a type that satisfies kdtree.Interface
type Points []Point

func (ps Points) Index(i int) kdtree.Comparable { return ps[i] }

func (ps Points) Len() int { return len(ps) }

func (ps Points) Slice(start, end int) kdtree.Interface { return ps[start:end] }

func (ps Points) Pivot(d kdtree.Dim) int {
	ph := &pointsHelper{Points: ps, Dim: d}
	return ph.Pivot()
}

// pointsHelper is required to help Points
type pointsHelper struct {
	kdtree.Dim
	Points
}

func (ph *pointsHelper) Less(i, j int) bool {
	switch p.Dim {
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

func (ph *pointsHelper) Pivot() int {
	return kdtree.Partition(ph, kdtree.MedianOfMedians(ph))
}

func (ph *pointsHelper) Slice(start, end int) kdtree.SortSlicer {
	ph.Points = ph.Points[start:end]
	return ph
}

func (ph *pointsHelper) Swap(i, j int) {
	ph.Points[i], ph.Points[j] = ph.Points[j], ph.Points[i]
}
