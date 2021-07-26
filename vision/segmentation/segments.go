package segmentation

import (
	pc "go.viam.com/core/pointcloud"
	"go.viam.com/core/utils"
)

// Cluster is a point cloud with respective meta-data, like center coordinate
type Cluster struct {
	pc.PointCloud
	Center pc.Vec3
}

// NewCluster creates a new cluster
func NewCluster(cloud pc.PointCloud, center pc.Vec3) *Cluster {
	return &Cluster{cloud, center}
}

// NewEmptyCluster creates a new empty cluster
func NewEmptyCluster() *Cluster {
	cloud := pc.New()
	center := pc.Vec3{}
	return &Cluster{cloud, center}
}

// Segments is a struct for keeping track of the individual segments of a point cloud as they are being built.
// Clusters is a slice of all the segments, and Indices is a map that assigns each point to the segment index it is a part of.
type Segments struct {
	Clusters []*Cluster
	Indices  map[pc.Vec3]int
}

// NewSegments creates an empty new Segments struct
func NewSegments() *Segments {
	segments := make([]*Cluster, 0)
	indices := make(map[pc.Vec3]int)
	return &Segments{segments, indices}
}

// NewSegmentsFromSlice creates a Segments struct from a slice of point clouds
func NewSegmentsFromSlice(clouds []pc.PointCloud) *Segments {
	segments := NewSegments()
	for i, cloud := range clouds {
		center := pc.CalculateMeanOfPointCloud(cloud)
		seg := &Cluster{cloud, center}
		segments.Clusters = append(segments.Clusters, seg)
		cloud.Iterate(func(pt pc.Point) bool {
			segments.Indices[pt.Position()] = i
			return true
		})
	}
	return segments
}

// N gives the number of clusters in the partition of the point cloud.
func (c *Segments) N() int {
	return len(c.Clusters)
}

// PointsClouds returns the underlying array of pointclouds
func (c *Segments) PointClouds() []pc.PointCloud {
	clouds := make([]pc.PointCloud, c.N())
	for i := 0; i < c.N(); i++ {
		clouds[i] = c.Clusters[i].PointCloud
	}
	return clouds
}

// AssignCluster assigns the given point to the cluster with the given index
func (c *Segments) AssignCluster(point pc.Point, index int) error {
	for index >= len(c.Clusters) {
		c.Clusters = append(c.Clusters, NewEmptyCluster())
	}
	n := float64(c.Clusters[index].Size())
	c.Indices[point.Position()] = index
	err := c.Clusters[index].Set(point)
	if err != nil {
		return err
	}
	// update center point
	pos := point.Position()
	c.Clusters[index].Center.X = (c.Clusters[index].Center.X*n + pos.X) / (n + 1)
	c.Clusters[index].Center.Y = (c.Clusters[index].Center.Y*n + pos.Y) / (n + 1)
	c.Clusters[index].Center.Z = (c.Clusters[index].Center.Z*n + pos.Z) / (n + 1)
	return nil
}

// MergeClusters moves all the points in index "from" to the segment at index "to"
func (c *Segments) MergeClusters(from, to int) error {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(c.Clusters) {
		c.Clusters = append(c.Clusters, NewEmptyCluster())
	}
	c.Clusters[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		n := float64(c.Clusters[to].Size())
		c.Indices[v] = to
		err = c.Clusters[to].Set(pt)
		// update center point
		c.Clusters[to].Center.X = (c.Clusters[to].Center.X*n + v.X) / (n + 1)
		c.Clusters[to].Center.Y = (c.Clusters[to].Center.Y*n + v.Y) / (n + 1)
		c.Clusters[to].Center.Z = (c.Clusters[to].Center.Z*n + v.Z) / (n + 1)
		c.Clusters[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	if err != nil {
		return err
	}
	c.Clusters[from].Center = pc.Vec3{}
	return nil
}
