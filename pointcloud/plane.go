package pointcloud

import (
	"github.com/golang/geo/r3"
)

// Plane defines a planar object in a 3D space.
type Plane interface {
	Equation() [4]float64            // Returns an array of the plane equation [0]x + [1]y + [2]z + [3] = 0.
	Normal() Vec3                    // The normal vector of the plane, could point "up" or "down".
	Center() Vec3                    // The center point of the plane (the planes are not infinite).
	Offset() float64                 //  the [3] term in the equation of the plane.
	PointCloud() (PointCloud, error) // Returns the underlying pointcloud that makes up the plane.
	Distance(p Vec3) float64         // The distance of a point p from the nearest point on the plane.
}

type pointcloudPlane struct {
	pointcloud PointCloud
	equation   [4]float64
	center     Vec3
}

// NewEmptyPlane initializes an empty plane object.
func NewEmptyPlane() Plane {
	return &pointcloudPlane{New(), [4]float64{}, Vec3{}}
}

// NewPlane creates a new plane object from a point cloud.
func NewPlane(cloud PointCloud, equation [4]float64) Plane {
	center := r3.Vector{}
	cloud.Iterate(func(pt Point) bool {
		center = center.Add(r3.Vector(pt.Position()))
		return true
	})
	if cloud.Size() != 0 {
		center = center.Mul(1. / float64(cloud.Size()))
	}
	return NewPlaneWithCenter(cloud, equation, Vec3(center))
}

// NewPlaneWithCenter creates a new plane object from a point cloud.
func NewPlaneWithCenter(cloud PointCloud, equation [4]float64, center Vec3) Plane {
	return &pointcloudPlane{cloud, equation, center}
}

// PointCloud returns the underlying point cloud of the plane.
func (p *pointcloudPlane) PointCloud() (PointCloud, error) {
	return p.pointcloud, nil
}

// Normal return the normal vector perpendicular to the plane.
func (p *pointcloudPlane) Normal() Vec3 {
	return Vec3{p.equation[0], p.equation[1], p.equation[2]}
}

// Center returns the vector pointing to the center of the points that make up the plane.
func (p *pointcloudPlane) Center() Vec3 {
	return p.center
}

// Offset returns the vector offset of the plane from the origin.
func (p *pointcloudPlane) Offset() float64 {
	return p.equation[3]
}

// Equation returns the plane equation [0]x + [1]y + [2]z + [3] = 0.
func (p *pointcloudPlane) Equation() [4]float64 {
	return p.equation
}

// Distance calculates the distance from the plane to the input point.
func (p *pointcloudPlane) Distance(pt Vec3) float64 {
	return (p.equation[0]*pt.X + p.equation[1]*pt.Y + p.equation[2]*pt.Z + p.equation[3]) / r3.Vector(pt).Norm()
}
