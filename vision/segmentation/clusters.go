package segmentation

import (
	pc "go.viam.com/core/pointcloud"
	"go.viam.com/core/utils"
)

// Clusters is a struct for keeping track of the individual segments of a point cloud as they are being built.
// PointClouds is a slice of all the segments, and Indices is a map that assigns each point to the segment index it is a part of.
type Clusters struct {
	PointClouds []pc.PointCloud
	Indices     map[pc.Vec3]int
}

// NewClusters creates an empty new Clusters struct
func NewClusters() *Clusters {
	pointclouds := make([]pc.PointCloud, 0)
	indices := make(map[pc.Vec3]int)
	return &Clusters{pointclouds, indices}
}

// N gives the number of clusters in the partition of the point cloud.
func (c *Clusters) N() int {
	return len(c.PointClouds)
}

// AssignCluster assigns the given point to the cluster with the given index
func (c *Clusters) AssignCluster(point pc.Point, index int) error {
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
	}
	c.Indices[point.Position()] = index
	err := c.PointClouds[index].Set(point)
	return err
}

// MergeClusters moves all the points in index "from" to the segment at index "to"
func (c *Clusters) MergeClusters(from, to int) error {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
	}
	c.PointClouds[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		c.Indices[v] = to
		err = c.PointClouds[to].Set(pt)
		c.PointClouds[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	return err
}
