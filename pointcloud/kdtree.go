package pointcloud

import "sort"

const dim = 3

type KDTree struct {
	PointCloud
	root *kdTreeNode
}

func NewKDTree(pc PointCloud) *KDTree {
	if pc.Size() == 0 {
		return nil
	}
	points := pc.Points()
	tree := &KDTree{
		PointCloud: pc,
		root:       createKDTree(points, 0),
	}
	return tree
}

type kdPoint struct {
	Point
}

type kdPoints []kdPoint

func (p kdPoint) Dims() int { return 3 }

func createKDTree(points []Point, depth int) *kdTreeNode {
	if len(points) == 0 {
		return nil
	}
	node := &kdTreeNode{
		axis: depth % dim,
	}
	if len(points) == 1 {
		node.splittingPoint = points[0]
		return node
	}
	sort.Sort(&splitHelper{dimension: axis, points: points})

	return node
}
