package segmentation

import (
	"fmt"

	"github.com/golang/geo/r3"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// Segments is a struct for keeping track of the individual objects of a point cloud as they are being built.
// Objects is a slice of all the objects, and Indices is a map that assigns each point to the object index it is a part of.
type Segments struct {
	Objects []*vision.Object
	Indices map[r3.Vector]int
}

// NewSegments creates an empty new Segments struct.
func NewSegments() *Segments {
	segments := make([]*vision.Object, 0)
	indices := make(map[r3.Vector]int)
	return &Segments{segments, indices}
}

// NewSegmentsFromSlice creates a Segments struct from a slice of point clouds.
func NewSegmentsFromSlice(clouds []pc.PointCloud, label string) (*Segments, error) {
	segments := NewSegments()
	for i, cloud := range clouds {
		seg, err := vision.NewObjectWithLabel(cloud, label, nil)
		if err != nil {
			return nil, err
		}
		segments.Objects = append(segments.Objects, seg)
		cloud.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
			segments.Indices[pt] = i
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
	v := r3.Vector{x, y, z}
	if segIndex, ok := c.Indices[v]; ok {
		return c.Objects[segIndex], nil
	}
	return nil, fmt.Errorf("no segment found at point (%v, %v, %v)", x, y, z)
}

// getPointCloudAndLabel returns the *vision.Object from `objs` at `idx` and the label from
// vision.Object.spatialmath.Geometry if the Geometry exists, or otherwise an empty string.
func getPointCloudAndLabel(objs []*vision.Object, idx int) (*vision.Object, string) {
	obj := objs[idx]
	if obj.Geometry != nil {
		return obj, obj.Geometry.Label()
	}
	return obj, ""
}

// AssignCluster assigns the given point to the cluster with the given index.
func (c *Segments) AssignCluster(point r3.Vector, data pc.Data, index int) error {
	for index >= len(c.Objects) {
		c.Objects = append(c.Objects, vision.NewEmptyObject())
	}
	c.Indices[point] = index
	err := c.Objects[index].Set(point, data)
	if err != nil {
		return err
	}
	if c.Objects[index].Size() == 0 {
		return nil
	}
	idx, label := getPointCloudAndLabel(c.Objects, index)
	c.Objects[index].Geometry, err = pc.BoundingBoxFromPointCloudWithLabel(idx, label)
	if err != nil {
		return err
	}
	return nil
}

// MergeClusters moves all the points in index "from" to the segment at index "to".
func (c *Segments) MergeClusters(from, to int) error {
	// ensure no out of bounds errors
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
	c.Objects[from].Iterate(0, 0, func(v r3.Vector, d pc.Data) bool {
		c.Indices[v] = to
		err = c.Objects[to].Set(v, d)
		return err == nil
	})
	if err != nil {
		return err
	}
	c.Objects[from] = vision.NewEmptyObject()
	// because BoundingBoxFromPointCloudWithLabel takes a PointCloud interface as its first argument, only the
	// vision.Object.pointcloud.PointCloud is passed to this method from our vision.Object, not the spatialmath.Geometry.
	// Consequently, it cannot access spatialmath.Geometry.Label(). Therefore, we must pass the label here before this
	// information is lost.
	t, label := getPointCloudAndLabel(c.Objects, to)
	c.Objects[to].Geometry, err = pc.BoundingBoxFromPointCloudWithLabel(t, label)
	if err != nil {
		return err
	}
	return nil
}
