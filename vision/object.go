package vision

import (
	"context"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// An ObjectSource3D is anything that generates 3D objects in a scene.
type ObjectSource3D interface {
	NextObjects(ctx context.Context, parameters *Parameters3D) ([]*Object, error)
}

// Object extends PointCloud with respective metadata, like the center coordinate.
// NOTE(bh):Can potentially add category or pose information to this struct.
type Object struct {
	pc.PointCloud
	BoundingBox spatialmath.Geometry
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
