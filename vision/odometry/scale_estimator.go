package odometry

import (
	"errors"
	"math"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
)

// remap3dFeatures remaps the y and z coordinates so that the y coordinate is the up-down coordinate and the
// z coordinate is the in-out coordinate, given a 3D feature vector
func remap3dFeatures(f3d []r3.Vector, pitch float64) []r3.Vector {
	remappedF3d := make([]r3.Vector, len(f3d))
	for i, pt := range f3d {
		y := pt.Y*math.Cos(pitch) - pt.Z*math.Sin(pitch)
		z := pt.Y*math.Sin(pitch) + pt.Z*math.Cos(pitch)
		remappedF3d[i] = r3.Vector{pt.X, y, z}
	}
	return remappedF3d
}

// getIdsFeaturesLowerThanVanish gets the ids of 2d features below vanshing height
func getIdsFeaturesLowerThanVanish(f2d []r2.Point, vanish float64) []int {
	ids := make([]int, 0, len(f2d))
	for i, pt := range f2d {
		if pt.Y > vanish {
			ids = append(ids, i)
		}
	}
	return ids
}

// getSelected2DFeatures returns the 2D features whose ids are selected
func getSelected2DFeatures(f2d []r2.Point, ids []int) []r2.Point {
	f2dSelected := make([]r2.Point, 0, len(f2d))
	for _, id := range ids {
		f2dSelected = append(f2dSelected, f2d[id])
	}
	return f2dSelected
}

// getSelected3DFeatures returns the 3D features whose ids are selected
func getSelected3DFeatures(f3d []r3.Vector, ids []int) []r3.Vector {
	f3dSelected := make([]r3.Vector, 0, len(f3d))
	for _, id := range ids {
		f3dSelected = append(f3dSelected, f3d[id])
	}
	return f3dSelected
}

// checkTriangle returns a mask for valid ground plane points ids
func checkTriangle(vs, ds []float64) ([]float64, error) {
	flags := []float64{0, 0, 0}
	if len(vs) != 3 {
		return nil, errors.New("triangle vertices should have a length of 3")
	}
	if len(ds) != 3 {
		return nil, errors.New("triangle direction should have a length of 3")
	}

	a := (vs[0] - vs[1]) * (ds[0] - ds[1])
	b := (vs[0] - vs[2]) * (ds[0] - ds[2])
	c := (vs[1] - vs[2]) * (ds[1] - ds[2])
	if a > 0 {
		flags[0], flags[1] = 1, 1
	}
	if b > 0 {
		flags[0], flags[2] = 1, 1
	}
	if c > 0 {
		flags[1], flags[2] = 1, 1
	}
	return flags, nil
}

// getTriangleVerticesDirection gets the 3D z components of the points in triangle, and the y components of 2D points
func getTriangleVerticesDirection(f2d []r2.Point, f3d []r3.Vector, trianglePointsID []int) ([]float64, []float64) {

	vs := []float64{f2d[trianglePointsID[0]].Y, f2d[trianglePointsID[1]].Y, f2d[trianglePointsID[2]].Y}
	depths := []float64{f3d[trianglePointsID[0]].Z, f3d[trianglePointsID[1]].Z, f3d[trianglePointsID[2]].Z}
	return vs, depths
}

// getTriangleNormalVector returns the normal vector of a 3D triangle
func getTriangleNormalVector(tri3d []r3.Vector) r3.Vector {
	u := tri3d[1].Sub(tri3d[0])
	v := tri3d[2].Sub(tri3d[0])
	normal := u.Cross(v)
	return normal
}
