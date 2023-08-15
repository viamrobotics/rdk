package vision

import (
	"errors"
	"math"


	commonpb "go.viam.com/api/common/v1"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// Object extends PointCloud with respective metadata, like the center coordinate.
// NOTE(bh):Can potentially add category or pose information to this struct.
type Object struct {
	pc.PointCloud
	Geometry spatialmath.Geometry
}

// NewObject creates a new vision.Object from a point cloud with an empty label.
func NewObject(cloud pc.PointCloud) (*Object, error) {
	return NewObjectWithLabel(cloud, "", nil)
}

// NewObjectWithLabel creates a new vision.Object from a point cloud with the given label.
func NewObjectWithLabel(cloud pc.PointCloud, label string, geometry *commonpb.Geometry) (*Object, error) {
	if cloud == nil {
		return NewEmptyObject(), nil
	}
	if geometry == nil {
		box, err := pc.BoundingBoxFromPointCloudWithLabel(cloud, label)
		if err != nil {
			return nil, err
		}
		return &Object{cloud, box}, nil
	}
	geom, err := spatialmath.NewGeometryFromProto(geometry)
	if err != nil {
		return nil, err
	}
	return &Object{PointCloud: cloud, Geometry: geom}, nil
}

// NewEmptyObject creates a new empty point cloud with metadata.
func NewEmptyObject() *Object {
	cloud := pc.New()
	return &Object{PointCloud: cloud}
}

// Distance calculates and returns the distance from the center point of the object to the origin.
func (o *Object) Distance() (float64, error) {
	if o.Geometry == nil {
		return -1, errors.New("no geometry object defined for distance formula to be applied")
	}
	point := o.Geometry.Pose().Point()
	dist := math.Pow(point.X, 2) + math.Pow(point.Y, 2) + math.Pow(point.Z, 2)
	dist = math.Sqrt(dist)
	return dist, nil
}
