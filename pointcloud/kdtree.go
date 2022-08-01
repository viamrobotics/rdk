package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/spatial/kdtree"
)

// PointAndData is a tiny struct to facilitate returning nearest neighbors in a neat way.
type PointAndData struct {
	P r3.Vector
	D Data
}

// wraps r3.vector to make it compatible with kd trees.
type treeComparableR3Vector struct {
	vec r3.Vector
}

func (v treeComparableR3Vector) Compare(c kdtree.Comparable, d kdtree.Dim) float64 {
	v2, ok := c.(treeComparableR3Vector)
	if !ok {
		panic("treeComparableR3Vector Compare got wrong data")
	}
	switch d {
	case 0:
		return v.vec.X - v2.vec.X
	case 1:
		return v.vec.Y - v2.vec.Y
	case 2:
		return v.vec.Z - v2.vec.Z
	default:
		panic("illegal dimension fed to treeComparableR3Vector.Compare")
	}
}

func (v treeComparableR3Vector) Dims() int {
	return 3
}

func (v treeComparableR3Vector) Distance(c kdtree.Comparable) float64 {
	v2, ok := c.(treeComparableR3Vector)
	if !ok {
		panic("treeComparableR3Vector Distance got wrong data")
	}
	return v.vec.Distance(v2.vec)
}

type kdValues []treeComparableR3Vector

func (vs kdValues) Index(i int) kdtree.Comparable { return vs[i] }

func (vs kdValues) Len() int { return len(vs) }

func (vs kdValues) Slice(start, end int) kdtree.Interface { return vs[start:end] }

func (vs kdValues) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

func (vs kdValues) Pivot(d kdtree.Dim) int {
	return kdValuesSlicer{vs: vs}.Pivot()
}

type kdValuesSlicer struct {
	vs kdValues
}

func (kdv kdValuesSlicer) Len() int { return len(kdv.vs) }

func (kdv kdValuesSlicer) Less(i, j int) bool {
	return kdv.vs[i].vec.Distance(kdv.vs[j].vec) < 0
}

func (kdv kdValuesSlicer) Pivot() int { return kdtree.Partition(kdv, kdtree.MedianOfMedians(kdv)) }
func (kdv kdValuesSlicer) Slice(start, end int) kdtree.SortSlicer {
	kdv.vs = kdv.vs[start:end]
	return kdv
}

func (kdv kdValuesSlicer) Swap(i, j int) {
	kdv.vs[i], kdv.vs[j] = kdv.vs[j], kdv.vs[i]
}

// ----------

// KDTree extends PointCloud and orders the points in 3D space to implement nearest neighbor algos.
type KDTree struct {
	tree   *kdtree.Tree
	points storage
	meta   MetaData
}

// NewKDTree creates a KDTree from an input PointCloud.
func NewKDTree(pc PointCloud) *KDTree {
	t := &KDTree{
		tree:   kdtree.New(kdValues{}, false),
		points: &matrixStorage{points: make([]PointAndData, pc.Size()), indexMap: make(map[r3.Vector]uint, pc.Size())},
		meta:   NewMetaData(),
	}

	if pc != nil {
		pc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			_, pointExists := t.At(p.X, p.Y, p.Z)
			err := t.Set(p, d)
			if err != nil {
				panic(err)
			}
			err = t.points.Set(p, d)
			if err != nil {
				panic(err)
			}
			if !pointExists {
				t.meta.Merge(p, d)
			}
			return true
		})
	}

	return t
}

// MetaData returns the meta data.
func (kd *KDTree) MetaData() MetaData {
	return kd.meta
}

// Size returns the size of the pointcloud.
func (kd *KDTree) Size() int {
	return kd.points.Size()
}

// Set adds a new point to the PointCloud and tree. Does not rebalance the tree.
func (kd *KDTree) Set(p r3.Vector, d Data) error {
	kd.tree.Insert(treeComparableR3Vector{p}, false)
	if err := kd.points.Set(p, d); err != nil {
		return err
	}
	kd.meta.Merge(p, d)
	return nil
}

// At gets the point at position (x,y,z) from the PointCloud.
// It returns the data of the nearest neighbor and a boolean representing whether there is a point at that position.
func (kd *KDTree) At(x, y, z float64) (Data, bool) {
	p, d, _, got := kd.NearestNeighbor(r3.Vector{x, y, z})
	if !got {
		return nil, false
	}
	if x == p.X && y == p.Y && z == p.Z {
		return d, true
	}
	return nil, false
}

// NearestNeighbor returns the nearest point and its distance from the input point.
func (kd *KDTree) NearestNeighbor(p r3.Vector) (r3.Vector, Data, float64, bool) {
	c, dist := kd.tree.Nearest(&treeComparableR3Vector{p})
	if c == nil {
		return r3.Vector{}, nil, 0.0, false
	}
	p2, ok := c.(treeComparableR3Vector)
	if !ok {
		panic("kdtree.Comparable is not a Point")
	}
	d, ok := kd.points.At(p2.vec.X, p2.vec.Y, p2.vec.Z)
	if !ok {
		panic("Mismatch between tree and point storage.")
	}
	return p2.vec, d, dist, true
}

func keeperToArray(heap kdtree.Heap, points storage, p r3.Vector, includeSelf bool, max int) []*PointAndData {
	nearestPoints := make([]*PointAndData, 0, heap.Len())
	for i := 0; i < heap.Len(); i++ {
		if heap[i].Comparable == nil {
			continue
		}
		pp, ok := heap[i].Comparable.(treeComparableR3Vector)
		if !ok {
			panic("impossible")
		}
		if !includeSelf && p.ApproxEqual(pp.vec) {
			continue
		}
		d, ok := points.At(pp.vec.X, pp.vec.Y, pp.vec.Z)
		if !ok {
			panic("Mismatch between tree and point storage.")
		}
		nearestPoints = append(nearestPoints, &PointAndData{P: pp.vec, D: d})
		if len(nearestPoints) >= max {
			break
		}
	}
	return nearestPoints
}

// KNearestNeighbors returns the k nearest points ordered by distance. if includeSelf is true and if the point p
// is in the point cloud, point p will also be returned in the slice as the first element with distance 0.
func (kd *KDTree) KNearestNeighbors(p r3.Vector, k int, includeSelf bool) []*PointAndData {
	tempK := k
	if !includeSelf {
		tempK++
	}

	keep := kdtree.NewNKeeper(tempK)
	kd.tree.NearestSet(keep, &treeComparableR3Vector{p})
	return keeperToArray(keep.Heap, kd.points, p, includeSelf, k)
}

// RadiusNearestNeighbors returns the nearest points within a radius r (inclusive) ordered by distance.
// If includeSelf is true and if the point p is in the point cloud, point p will also be returned in the slice
// as the first element with distance 0.
func (kd *KDTree) RadiusNearestNeighbors(p r3.Vector, r float64, includeSelf bool) []*PointAndData {
	keep := kdtree.NewDistKeeper(r)
	kd.tree.NearestSet(keep, &treeComparableR3Vector{p})
	return keeperToArray(keep.Heap, kd.points, p, includeSelf, math.MaxInt)
}

// Iterate iterates over all points in the cloud.
func (kd *KDTree) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	kd.tree.Do(func(c kdtree.Comparable, b *kdtree.Bounding, depth int) bool {
		p, ok := c.(treeComparableR3Vector)
		if !ok {
			panic("Comparable is not a Point")
		}
		d, ok := kd.points.At(p.vec.X, p.vec.Y, p.vec.Z)
		if !ok {
			panic("Mismatch between tree and point storage.")
		}
		return !fn(p.vec, d)
	})
}
