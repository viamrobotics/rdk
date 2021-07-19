package pointcloud

import (
	"github.com/golang/geo/r3"
)

// Plane defines a planar object in a point cloud
type Plane interface {
	Normal() Vec3
	Center() Vec3
	Offset() float64
	PointCloud() (PointCloud, error)
	Equation() []float64
	Distance(p Vec3) float64
}

type pointcloudPlane struct {
	pointcloud PointCloud
	equation   []float64
	center     *Vec3
}

// NewEmptyPlane initializes an empty plane object
func NewEmptyPlane() Plane {
	return &pointcloudPlane{New(), []float64{0, 0, 0, 0}, nil}
}

func NewPlane(cloud PointCloud, equation []float64) Plane {
	return &pointcloudPlane{cloud, equation, nil}
}

// PointCloud returns the underlying point cloud of the plane
func (p *pointcloudPlane) PointCloud() (PointCloud, error) {
	return p.pointcloud, nil
}

func (p *pointcloudPlane) Normal() Vec3 {
	return Vec3{p.equation[0], p.equation[1], p.equation[2]}
}

func (p *pointcloudPlane) Center() Vec3 {
	if p.center != nil {
		return *p.center
	}
	center := r3.Vector{}
	p.pointcloud.Iterate(func(pt Point) bool {
		center.Add(r3.Vector(pt.Position()))
		return true
	})
	center = center.Mul(1. / float64(p.pointcloud.Size()))
	centerVec3 := Vec3(center)
	p.center = &centerVec3 // cache the result
	return *p.center
}

func (p *pointcloudPlane) Offset() float64 {
	return p.equation[3]
}

// Equation returns the plane equation [0]x + [1]y + [2]z + [3] = 0.
func (p *pointcloudPlane) Equation() []float64 {
	return p.equation
}

// Distance calculates the distance from the plane to the input point
func (p *pointcloudPlane) Distance(pt Vec3) float64 {
	return (p.equation[0]*pt.X + p.equation[1]*pt.Y + p.equation[2]*pt.Z + p.equation[3]) / r3.Vector(pt).Norm()
}
