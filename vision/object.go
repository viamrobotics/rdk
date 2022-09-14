package vision

import (
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// Object extends PointCloud with respective metadata, like the center coordinate.
// NOTE(bh):Can potentially add category or pose information to this struct.
type Object struct {
	pc.PointCloud
	Geometry spatialmath.Geometry
}

// NewObject calculates the metadata for an input pointcloud.
func NewObject(cloud pc.PointCloud) (*Object, error) {
	if cloud == nil {
		return NewEmptyObject(), nil
	}
	box, err := pc.BoundingBoxFromPointCloud(cloud)
	if err != nil {
		return nil, err
	}
	return &Object{cloud, box}, nil
}

// NewEmptyObject creates a new empty point cloud with metadata.
func NewEmptyObject() *Object {
	cloud := pc.New()
	return &Object{PointCloud: cloud}
}
