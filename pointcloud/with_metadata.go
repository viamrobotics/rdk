package pointcloud

// WithMetadata extends PointCloud with respective meta-data, like center coordinate
// Can potentially add category or pose information to this struct.
type WithMetadata struct {
	PointCloud
	Center      Vec3
	BoundingBox BoxGeometry
}

// NewWithMetadata calculates the metadata for an input pointcloud.
func NewWithMetadata(cloud PointCloud) *WithMetadata {
	if cloud == nil {
		return NewEmptyWithMetadata()
	}
	center := CalculateMeanOfPointCloud(cloud)
	boundingBox := CalculateBoundingBoxOfPointCloud(cloud)
	return &WithMetadata{cloud, center, boundingBox}
}

// NewEmptyWithMetadata creates a new empty point cloud with metadata.
func NewEmptyWithMetadata() *WithMetadata {
	cloud := New()
	center := Vec3{}
	return &WithMetadata{PointCloud: cloud, Center: center}
}
