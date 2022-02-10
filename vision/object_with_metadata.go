package vision

import (
	pc "go.viam.com/rdk/pointcloud"
)

// Object extends PointCloud with respective meta-data, like center coordinate
// Can potentially add category or pose information to this struct.
type Object struct {
	pc.PointCloud
	Center      pc.Vec3
	BoundingBox pc.BoxGeometry
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
