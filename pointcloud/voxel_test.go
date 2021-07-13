package pointcloud

import (
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestVoxelCoords(t *testing.T) {
	// Test creation
	c1 := VoxelCoords{}
	test.ShouldEqual(c1.I, 0)
	test.ShouldEqual(c1.J, 0)
	test.ShouldEqual(c1.K, 0)
	// Test IsEqual function
	c2 := VoxelCoords{2, 1, 3}
	c3 := VoxelCoords{2, 1, 3}
	test.ShouldBeTrue(c2.IsEqual(c3))
}

func TestVoxelCreation(t *testing.T) {

	pt := r3.Vector{
		X: 1.2,
		Y: 0.5,
		Z: 2.8,
	}
	ptMin := r3.Vector{
		X: 0,
		Y: 0,
		Z: 0,
	}
	voxelSize := 1.0
	vox := NewVoxelFromPoint(pt, ptMin, voxelSize)
	test.ShouldEqual(vox.Key.I, 1)
	test.ShouldEqual(vox.Key.J, 0)
	test.ShouldEqual(vox.Key.I, 2)
	test.ShouldEqual(vox.Label, 0)
	test.ShouldEqual(vox.PointLabels, nil)

	vox.SetLabel(10)
	test.ShouldEqual(vox.Label, 10)
}

func TestVoxelGridCreation(t *testing.T) {
	nPoints := 1000000
	pc := GenerateCubeTestData(nPoints)
	vg := NewVoxelGridFromPointCloud(pc, 0.1, 0.01)
	test.ShouldEqual(len(vg.Voxels), 571)
	test.ShouldEqual(vg.maxLabel, 0)
}

func TestEstimatePlaneNormalFromPoints(t *testing.T) {
	nPoints := 1000
	points := make([]r3.Vector, nPoints)
	for i := 0; i < nPoints; i++ {
		// Point in the R3 unit cube, on plane z=0
		p := r3.Vector{rand.Float64(), rand.Float64(), 0}
		points = append(points, p)
	}
	normalPlane := estimatePlaneNormalFromPoints(points)
	test.ShouldAlmostEqual(normalPlane.X, 0.)
	test.ShouldAlmostEqual(normalPlane.Y, 0.)
	test.ShouldAlmostEqual(normalPlane.Z, 1.)
}

func TestGetVoxelCenterWeightResidual(t *testing.T) {
	nPoints := 10000
	points := make([]r3.Vector, nPoints)
	for i := 0; i < nPoints; i++ {
		// Point in the R3 unit cube, on plane z=0
		p := r3.Vector{rand.Float64(), rand.Float64(), 0}
		points = append(points, p)
	}
	center := GetVoxelCenter(points)
	test.ShouldAlmostEqual(center.X, 0.5)
	test.ShouldAlmostEqual(center.Y, 0.5)
	test.ShouldAlmostEqual(center.Z, 0.)

	w := GetWeight(points, 1., 0.)
	test.ShouldAlmostEqual(w, 1.0)
	plane := Plane{
		Normal:    r3.Vector{0, 0, 1},
		Center:    r3.Vector{},
		Offset:    0,
		Points:    nil,
		VoxelKeys: nil,
	}
	res := GetResidual(points, plane)
	test.ShouldAlmostEqual(res, 0.0)
}

func TestGetVoxelCoordinates(t *testing.T) {
	// Get point in [0,1]x[0,1]x0
	p := r3.Vector{rand.Float64(), rand.Float64(), 0}
	ptMin := r3.Vector{}
	// if voxel of size 1, voxel coordinates should be (0,0,0)
	coords := GetVoxelCoordinates(p, ptMin, 1.0)
	test.ShouldAlmostEqual(coords.I, 0.)
	test.ShouldAlmostEqual(coords.J, 0.)
	test.ShouldAlmostEqual(coords.K, 0.)
}
