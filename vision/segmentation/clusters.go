package segmentation

import (
	pc "go.viam.com/core/pointcloud"
	"go.viam.com/core/utils"
)

// Clusters is a struct for keeping track of the individual segments of a point cloud as they are being built.
// PointClouds is a slice of all the segments, and Indices is a map that assigns each point to the segment index it is a part of.
type Clusters struct {
	PointClouds []pc.PointCloud
	Centers     []pc.Vec3
	Indices     map[pc.Vec3]int
}

// NewClusters creates an empty new Clusters struct
func NewClusters() *Clusters {
	pointclouds := make([]pc.PointCloud, 0)
	centers := make([]pc.Vec3, 0)
	indices := make(map[pc.Vec3]int)
	return &Clusters{pointclouds, centers, indices}
}

// NewClustersFromSlice creates a Clusters struct from a slice of point clouds
func NewClustersFromSlice(clouds []pc.PointCloud) *Clusters {
	indices := make(map[pc.Vec3]int)
	centers := make([]pc.Vec3, len(clouds))
	for i, cloud := range clouds {
		centers[i] = pc.CalculateMeanOfPointCloud(cloud)
		cloud.Iterate(func(pt pc.Point) bool {
			indices[pt.Position()] = i
			return true
		})
	}
	return &Clusters{clouds, centers, indices}
}

// N gives the number of clusters in the partition of the point cloud.
func (c *Clusters) N() int {
	return len(c.PointClouds)
}

// AssignCluster assigns the given point to the cluster with the given index
func (c *Clusters) AssignCluster(point pc.Point, index int) error {
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
		c.Centers = append(c.Centers, pc.Vec3{})
	}
	n := float64(c.PointClouds[index].Size())
	c.Indices[point.Position()] = index
	err := c.PointClouds[index].Set(point)
	if err != nil {
		return err
	}
	// update center point
	pos := point.Position()
	c.Centers[index].X = (c.Centers[index].X*n + pos.X) / (n + 1)
	c.Centers[index].Y = (c.Centers[index].Y*n + pos.Y) / (n + 1)
	c.Centers[index].Z = (c.Centers[index].Z*n + pos.Z) / (n + 1)
	return nil
}

// MergeClusters moves all the points in index "from" to the segment at index "to"
func (c *Clusters) MergeClusters(from, to int) error {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(c.PointClouds) {
		c.PointClouds = append(c.PointClouds, pc.New())
		c.Centers = append(c.Centers, pc.Vec3{})
	}
	c.PointClouds[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		n := float64(c.PointClouds[to].Size())
		c.Indices[v] = to
		err = c.PointClouds[to].Set(pt)
		// update center point
		c.Centers[to].X = (c.Centers[to].X*n + v.X) / (n + 1)
		c.Centers[to].Y = (c.Centers[to].Y*n + v.Y) / (n + 1)
		c.Centers[to].Z = (c.Centers[to].Z*n + v.Z) / (n + 1)
		c.PointClouds[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	if err != nil {
		return err
	}
	c.Centers[from] = pc.Vec3{}
	return nil
}
