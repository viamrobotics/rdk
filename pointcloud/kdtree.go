package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/spatial/kdtree"
)

type kdValues []PointAndData

func (vs kdValues) Index(i int) kdtree.Comparable { return vs[i] }

func (vs kdValues) Len() int { return len(vs) }

func (vs kdValues) Slice(start, end int) kdtree.Interface { return vs[start:end] }

func (vs kdValues) Pivot(d kdtree.Dim) int {
	panic("what")
}

// ----------

// KDTree extends PointCloud and orders the points in 3D space to implement nearest neighbor algos.
type KDTree struct {
	tree     *kdtree.Tree
	metadata MetaData
}

// NewKDTree creates a KDTree from an input PointCloud.
func NewKDTree(pc PointCloud) *KDTree {
	t := &KDTree{
		tree:     kdtree.New(kdValues{}, false),
		metadata: NewMetaData(),
	}

	if pc != nil {
		pc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			_, pointExists := t.At(p.X, p.Y, p.Z)
			err := t.Set(p, d)
			if err != nil {
				panic(err)
			}
			if !pointExists {
				t.metadata.Merge(p, d)
			}
			return true
		})
	}

	return t
}

// MetaData returns the meta data.
func (kd *KDTree) MetaData() MetaData {
	return kd.metadata
}

// Size returns the size of the pointcloud.
func (kd *KDTree) Size() int {
	return kd.tree.Len()
}

// Set adds a new point to the PointCloud and tree. Does not rebalance the tree.
func (kd *KDTree) Set(p r3.Vector, d Data) error {
	kd.tree.Insert(&PointAndData{p, d}, false)
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
	c, dist := kd.tree.Nearest(&PointAndData{p, nil})
	if c == nil {
		return r3.Vector{}, nil, 0.0, false
	}
	p2, ok := c.(*PointAndData)
	if !ok {
		panic("kdtree.Comparable is not a Point")
	}
	return p2.P, p2.D, dist, true
}

func keeperToArray(heap kdtree.Heap, p r3.Vector, includeSelf bool, max int) []*PointAndData {
	nearestPoints := make([]*PointAndData, 0, heap.Len())
	for i := 0; i < heap.Len(); i++ {
		if heap[i].Comparable == nil {
			continue
		}
		pp, ok := heap[i].Comparable.(*PointAndData)
		if !ok {
			panic("impossible")
		}
		if !includeSelf && p.ApproxEqual(pp.P) {
			continue
		}
		nearestPoints = append(nearestPoints, pp)
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
	kd.tree.NearestSet(keep, &PointAndData{p, nil})
	return keeperToArray(keep.Heap, p, includeSelf, k)
}

// RadiusNearestNeighbors returns the nearest points within a radius r (inclusive) ordered by distance.
// If includeSelf is true and if the point p is in the point cloud, point p will also be returned in the slice
// as the first element with distance 0.
func (kd *KDTree) RadiusNearestNeighbors(p r3.Vector, r float64, includeSelf bool) []*PointAndData {
	keep := kdtree.NewDistKeeper(r)
	kd.tree.NearestSet(keep, &PointAndData{p, nil})
	return keeperToArray(keep.Heap, p, includeSelf, math.MaxInt)
}

// Iterate iterates over all points in the cloud.
func (kd *KDTree) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	kd.tree.Do(func(c kdtree.Comparable, b *kdtree.Bounding, depth int) bool {
		x, ok := c.(*PointAndData)
		if !ok {
			panic("impossible")
		}
		return !fn(x.P, x.D)
	})
}
