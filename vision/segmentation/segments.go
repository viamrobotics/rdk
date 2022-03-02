package segmentation

import (
	"fmt"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// Segments is a struct for keeping track of the individual objects of a point cloud as they are being built.
// Objects is a slice of all the objects, and Indices is a map that assigns each point to the object index it is a part of.
type Segments struct {
	Objects []*vision.Object
	Indices map[pc.Vec3]int
}

// NewSegments creates an empty new Segments struct.
func NewSegments() *Segments {
	segments := make([]*vision.Object, 0)
	indices := make(map[pc.Vec3]int)
	return &Segments{segments, indices}
}

// NewSegmentsFromSlice creates a Segments struct from a slice of point clouds.
func NewSegmentsFromSlice(clouds []pc.PointCloud) (*Segments, error) {
	segments := NewSegments()
	for i, cloud := range clouds {
		seg, err := vision.NewObject(cloud)
		if err != nil {
			return nil, err
		}
		segments.Objects = append(segments.Objects, seg)
		cloud.Iterate(func(pt pc.Point) bool {
			segments.Indices[pt.Position()] = i
			return true
		})
	}
	return segments, nil
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
		c.Objects = append(c.Objects, vision.NewEmptyObject())
	}
	c.Indices[point.Position()] = index
	err := c.Objects[index].Set(point)
	if err != nil {
		return err
	}
	if c.Objects[index].Size() == 0 {
		return nil
	}
	c.Objects[index].BoundingBox, err = pc.BoundingBoxFromPointCloud(c.Objects[index])
	if err != nil {
		return err
	}
	return nil
}

// MergeClusters moves all the points in index "from" to the segment at index "to".
func (c *Segments) MergeClusters(from, to int) error {
	// ensure no out of bounrs errors
	index := utils.MaxInt(from, to)
	for index >= len(c.Objects) {
		c.Objects = append(c.Objects, vision.NewEmptyObject())
	}

	// if no objects are in the cluster to delete, just return
	if c.Objects[from].Size() == 0 {
		return nil
	}

	// perform merge
	var err error
	c.Objects[from].Iterate(func(pt pc.Point) bool {
		v := pt.Position()
		c.Indices[v] = to
		err = c.Objects[to].Set(pt)
		c.Objects[from].Unset(v.X, v.Y, v.Z)
		return err == nil
	})
	if err != nil {
		return err
	}
	c.Objects[from] = vision.NewEmptyObject()
	c.Objects[to].BoundingBox, err = pc.BoundingBoxFromPointCloud(c.Objects[to])
	if err != nil {
		return err
	}
	return nil
}
