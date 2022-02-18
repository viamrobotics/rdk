package segmentation

import (
	"fmt"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
)

// PointCloudWithMeta extends PointCloud with respective meta-data, like center coordinate
// Can potentially add category or pose information to this struct.
type PointCloudWithMeta struct {
	pc.PointCloud
	Center      pc.Vec3
	BoundingBox pc.RectangularPrism
}

// NewPointCloudWithMeta calculates the metadata for an input pointcloud.
func NewPointCloudWithMeta(cloud pc.PointCloud) *PointCloudWithMeta {
	center := pc.CalculateMeanOfPointCloud(cloud)
	boundingBox := pc.CalculateBoundingBoxOfPointCloud(cloud)
	return &PointCloudWithMeta{cloud, center, boundingBox}
}

// NewEmptyPointCloudWithMeta creates a new empty point cloud with metadata.
func NewEmptyPointCloudWithMeta() *PointCloudWithMeta {
	cloud := pc.New()
	center := pc.Vec3{}
	return &PointCloudWithMeta{PointCloud: cloud, Center: center}
}

// Segments is a struct for keeping track of the individual objects of a point cloud as they are being built.
// Objects is a slice of all the objects, and Indices is a map that assigns each point to the object index it is a part of.
type Segments struct {
	Objects []*PointCloudWithMeta
	Indices map[pc.Vec3]int
}

// NewSegments creates an empty new Segments struct.
func NewSegments() *Segments {
	segments := make([]*PointCloudWithMeta, 0)
	indices := make(map[pc.Vec3]int)
	return &Segments{segments, indices}
}

// NewSegmentsFromSlice creates a Segments struct from a slice of point clouds.
func NewSegmentsFromSlice(clouds []pc.PointCloud) *Segments {
	segments := NewSegments()
	for i, cloud := range clouds {
		seg := NewPointCloudWithMeta(cloud)
		segments.Objects = append(segments.Objects, seg)
		cloud.Iterate(func(pt pc.Point) bool {
			segments.Indices[pt.Position()] = i
			return true
		})
	}
	return segments
}

// N gives the number of objects in the partition of the point cloud.
func (c *Segments) N() int {
	return len(c.Objects)
}

// PointClouds returns the underlying array of pointclouds.
func (c *Segments) PointClouds() []pc.PointCloud {
	clouds := make([]pc.PointCloud, c.N())
	for i := 0; i < c.N(); i++ {
		clouds[i] = c.Objects[i]
	}
	return clouds
}

// SelectPointCloudFromPoint takes a 3D point as input and outputs the point cloud of the segment that the point belongs to.
func (c *Segments) SelectPointCloudFromPoint(x, y, z float64) (pc.PointCloud, error) {
	v := pc.Vec3{x, y, z}
	if segIndex, ok := c.Indices[v]; ok {
		return c.Objects[segIndex], nil
	}
	return nil, fmt.Errorf("no segment found at point (%v, %v, %v)", x, y, z)
}

// AssignCluster assigns the given point to the cluster with the given index.
func (c *Segments) AssignCluster(point pc.Point, index int) error {
	for index >= len(c.Objects) {
		c.Objects = append(c.Objects, NewEmptyPointCloudWithMeta())
	}
	n := float64(c.Objects[index].Size())
	c.Indices[point.Position()] = index
	err := c.Objects[index].Set(point)
	if err != nil {
		return err
	}
	// update center point
	pos := point.Position()
	c.Objects[index].Center.X = (c.Objects[index].Center.X*n + pos.X) / (n + 1)
	c.Objects[index].Center.Y = (c.Objects[index].Center.Y*n + pos.Y) / (n + 1)
	c.Objects[index].Center.Z = (c.Objects[index].Center.Z*n + pos.Z) / (n + 1)
	return nil
}

// MergeClusters moves all the points in index "from" to the segment at index "to".
func (c *Segments) MergeClusters(from, to int) error {
	var err error
	index := utils.MaxInt(from, to)
	for index >= len(c.Objects) {
		c.Objects = append(c.Objects, NewEmptyPointCloudWithMeta())
	}
	c.Objects[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		n := float64(c.Objects[to].Size())
		c.Indices[v] = to
		err = c.Objects[to].Set(pt)
		// update center point
		c.Objects[to].Center.X = (c.Objects[to].Center.X*n + v.X) / (n + 1)
		c.Objects[to].Center.Y = (c.Objects[to].Center.Y*n + v.Y) / (n + 1)
		c.Objects[to].Center.Z = (c.Objects[to].Center.Z*n + v.Z) / (n + 1)
		c.Objects[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	if err != nil {
		return err
	}
	c.Objects[from].Center = pc.Vec3{}
	return nil
}
