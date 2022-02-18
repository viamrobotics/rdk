package vision

import (
	"context"

	pc "go.viam.com/rdk/pointcloud"
)

// An ObjectSource3D is anything that generates 3D objects in a scene.
type ObjectSource3D interface {
	NextObjects(ctx context.Context, parameters *Parameters3D) ([]*Object, error)
}

// Object extends PointCloud with respective metadata, like the center coordinate.
// NOTE(bh):Can potentially add category or pose information to this struct.
type Object struct {
	pc.PointCloud
	Center      pc.Vec3
	BoundingBox pc.RectangularPrism
}

// NewObject calculates the metadata for an input pointcloud.
func NewObject(cloud pc.PointCloud) *Object {
	if cloud == nil {
		return NewEmptyObject()
	}
	center := pc.CalculateMeanOfPointCloud(cloud)
	boundingBox := pc.CalculateBoundingBoxOfPointCloud(cloud)
	return &Object{cloud, center, boundingBox}
}

// NewEmptyObject creates a new empty point cloud with metadata.
func NewEmptyObject() *Object {
	cloud := pc.New()
	center := pc.Vec3{}
	return &Object{PointCloud: cloud, Center: center}
}
