package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// helpers for Voxel attributes computation

// estimatePlaneNormalFromPoints estimates the normal vector of the plane formed by the points in the []r3.Vector.
func estimatePlaneNormalFromPoints(points []r3.Vector) r3.Vector {
	// Put points in mat
	nPoints := len(points)
	mPt := mat.NewDense(nPoints, 3, nil)
	for i, v := range points {
		mPt.Set(i, 0, v.X)
		mPt.Set(i, 1, v.Y)
		mPt.Set(i, 2, v.Z)
	}
	// Compute PCA
	var pc stat.PC
	ok := pc.PrincipalComponents(mPt, nil)
	if !ok {
		return r3.Vector{}
	}
	var vecs mat.Dense
	pc.VectorsTo(&vecs)
	// vectors are ordered by decreasing eigenvalues
	// the normal vector corresponds to the vector associated with the smallest eigenvalue
	// ie the last column in the vecs 3x3 matrix
	normalData := vecs.ColView(2)
	normal := r3.Vector{
		X: normalData.At(0, 0),
		Y: normalData.At(1, 0),
		Z: normalData.At(2, 0),
	}
	orientation := r3.Vector{1., 1., 1.}
	// orient normal vectors consistently
	if normal.Dot(orientation) < 0. {
		normal = normal.Mul(-1.0)
	}
	return normal.Normalize()
}

// GetVoxelCenter computes the barycenter of the points in the slice of r3.Vector.
func GetVoxelCenter(points []r3.Vector) r3.Vector {
	center := r3.Vector{}
	for _, pt := range points {
		center.X += pt.X
		center.Y += pt.Y
		center.Z += pt.Z
	}
	center = center.Mul(1. / float64(len(points)))
	return center
}

// GetOffset computes the offset of the plane with given normal vector and a point in it.
func GetOffset(center, normal r3.Vector) float64 {
	return -normal.Dot(center)
}

// GetResidual computes the mean fitting error of points to a given plane.
func GetResidual(points []r3.Vector, plane Plane) float64 {
	dist := 0.
	for _, pt := range points {
		d := plane.Distance(pt)
		dist += d * d
	}
	dist /= float64(len(points))
	return math.Sqrt(dist)
}

// GetVoxelCoordinates computes voxel coordinates in VoxelGrid Axes.
func GetVoxelCoordinates(pt, ptMin r3.Vector, voxelSize float64) VoxelCoords {
	ptVoxel := pt.Sub(ptMin)
	coords := VoxelCoords{}
	coords.I = int64(math.Floor(ptVoxel.X / voxelSize))
	coords.J = int64(math.Floor(ptVoxel.Y / voxelSize))
	coords.K = int64(math.Floor(ptVoxel.Z / voxelSize))
	return coords
}

// GetWeight computes weights for Region Growing segmentation.
func GetWeight(points []r3.Vector, lam, residual float64) float64 {
	nPoints := len(points)
	dR := 1. / float64(nPoints)
	w := math.Exp(-dR*dR/(2*lam*lam)) * math.Exp(-residual*residual/(2*lam*lam))
	return w
}
