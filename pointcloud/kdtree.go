package pointcloud

import "gonum.org/v1/gonum/spatial/kdtree"

type kdValue struct {
	p Vec3
	d Data
}

func (v kdValue) Compare(kdtree.Comparable, kdtree.Dim) float64 {
	panic(1)
}

func (v kdValue) Dims() int {
	return 3
}

func (v kdValue) Distance(kdtree.Comparable) float64 {
	panic(12)
}

type kdValues []kdValue

func (vs kdValues) Index(i int) kdtree.Comparable { return vs[i] }

func (vs kdValues) Len() int { return len(vs) }

func (vs kdValues) Slice(start, end int) kdtree.Interface { return vs[start:end] }

func (vs kdValues) Pivot(d kdtree.Dim) int {
	panic("what")
}

// ----------

// KDTree extends PointCloud and orders the points in 3D space to implement nearest neighbor algos.
type KDTree struct {
	tree    *kdtree.Tree
	rebuild bool
	toRemove []Vec3
}

// NewKDTree creates a KDTree from an input PointCloud.
func NewKDTree(pc PointCloud) *KDTree {

	t := &KDTree{kdtree.New(kdValues{}, false), false, nil}

	if pc != nil {
		pc.Iterate(0,0,func(p Vec3, d Data) bool {
			err := t.Set(p, d)
			if err != nil {
				panic(err)
			}
			return true
		})
	}

	return t
}

func (kd *KDTree) rebuildIfNeeded() {
	if !kd.rebuild {
		return
	}

	
	n := kdtree.New(kdValues{}, false)
	panic(1)
	// iterate
	// don't add if in unset
	kd.toRemove = []Vec3{}
	kd.rebuild = false
}

// Set adds a new point to the PointCloud and tree. Does not rebalance the tree.
func (kd *KDTree) Set(p Vec3, d Data) error {
	kd.tree.Insert(&kdValue{p, d}, false)
	return nil
}

// Unset removes the point from the PointCloud and sets a flag to rebuild the tree next time NN is used.
func (kd *KDTree) Unset(x, y, z float64) {
	kd.toRemove = append(kd.toRemove, Vec3{x,y,z})
	kd.rebuild = true
}

// NearestNeighbor returns the nearest point and its distance from the input point.
func (kd *KDTree) NearestNeighbor(p Vec3) (Vec3, Data, float64) {
	kd.rebuildIfNeeded()
	c, dist := kd.tree.Nearest(&kdValue{p, nil})
	if c == nil {
		return Vec3{}, nil, 0.0
	}
	p2, ok := c.(kdValue)
	if !ok {
		panic("kdtree.Comparable is not a Point")
	}
	return p2.p, p2.d, dist
}

// KNearestNeighbors returns the k nearest points ordered by distance. if includeSelf is true and if the point p
// is in the point cloud, point p will also be returned in the slice as the first element with distance 0.
func (kd *KDTree) KNearestNeighbors(p Point, k int, includeSelf bool) []Point {
	kd.rebuildIfNeeded()
	start := 0
	if kd.At(p.Position().X, p.Position().Y, p.Position().Z) != nil && !includeSelf {
		k++
		start++
	}
	keep := kdtree.NewNKeeper(k)
	kd.tree.NearestSet(keep, p)
	if keep.Heap.Len() == 1 && keep.Heap[0].Comparable == nil { // empty heap
		return []Point{}
	}
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
	kd.rebuildIfNeeded()
	start := 0
	if kd.At(p.Position().X, p.Position().Y, p.Position().Z) != nil && !includeSelf {
		start++
	}
	keep := kdtree.NewDistKeeper(r)
	kd.tree.NearestSet(keep, p)
	if keep.Heap.Len() == 1 && keep.Heap[0].Comparable == nil { // empty heap
		return []Point{}
	}
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
