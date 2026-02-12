package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
)

// Plane defines a planar object in a 3D space.
type Plane interface {
	Equation() [4]float64                  // Returns an array of the plane equation [0]x + [1]y + [2]z + [3] = 0.
	Normal() r3.Vector                     // The normal vector of the plane, could point "up" or "down".
	Center() r3.Vector                     // The center point of the plane (the planes are not infinite).
	Offset() float64                       //  the [3] term in the equation of the plane.
	PointCloud() (PointCloud, error)       // Returns the underlying pointcloud that makes up the plane.
	Distance(p r3.Vector) float64          // The distance of a point p from the nearest point on the plane.
	Intersect(p0, p1 r3.Vector) *r3.Vector // The intersection point of the plane with line defined by p0,p1. return nil if parallel.
}

type pointcloudPlane struct {
	pointcloud PointCloud
	equation   [4]float64
	center     r3.Vector
}

// NewEmptyPlane initializes an empty plane object.
func NewEmptyPlane(cloud PointCloud) Plane {
	return &pointcloudPlane{cloud, [4]float64{}, r3.Vector{}}
}

// NewPlane creates a new plane object from a point cloud.
func NewPlane(cloud PointCloud, equation [4]float64) Plane {
	center := r3.Vector{}
	cloud.Iterate(0, 0, func(pt r3.Vector, d Data) bool {
		center = center.Add(pt)
		return true
	})
	if cloud.Size() != 0 {
		center = center.Mul(1. / float64(cloud.Size()))
	}
	return NewPlaneWithCenter(cloud, equation, center)
}

// NewPlaneWithCenter creates a new plane object from a point cloud.
func NewPlaneWithCenter(cloud PointCloud, equation [4]float64, center r3.Vector) Plane {
	return &pointcloudPlane{cloud, equation, center}
}

// PointCloud returns the underlying point cloud of the plane.
func (p *pointcloudPlane) PointCloud() (PointCloud, error) {
	return p.pointcloud, nil
}

// Normal return the normal vector perpendicular to the plane.
func (p *pointcloudPlane) Normal() r3.Vector {
	return r3.Vector{p.equation[0], p.equation[1], p.equation[2]}
}

// Center returns the vector pointing to the center of the points that make up the plane.
func (p *pointcloudPlane) Center() r3.Vector {
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
func (p *pointcloudPlane) Distance(pt r3.Vector) float64 {
	return (p.equation[0]*pt.X + p.equation[1]*pt.Y + p.equation[2]*pt.Z + p.equation[3]) / pt.Norm()
}

// Intersect calculates the intersection point of the plane with line defined by p0,p1. return nil if parallel.
func (p *pointcloudPlane) Intersect(p0, p1 r3.Vector) *r3.Vector {
	line := p1.Sub(p0)
	parallel := line.Dot(p.Normal())
	if math.Abs(parallel) < 1e-6 { // the normal and line are perpendicular, will not intersect
		return nil
	}
	w := p0.Sub(p.center)
	fac := -w.Dot(p.Normal()) / parallel
	result := p0.Add(line.Mul(fac))
	return &result
}
