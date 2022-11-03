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
	nPoints := 10000
	pc := GenerateCubeTestData(nPoints)
	vg := NewVoxelGridFromPointCloud(pc, 0.5, 0.01)
	test.ShouldEqual(len(vg.Voxels), 571)
	test.ShouldEqual(vg.maxLabel, 0)
}

func TestVoxelGridCubeSegmentation(t *testing.T) {
	nPoints := 10000
	pc := GenerateCubeTestData(nPoints)
	vg := NewVoxelGridFromPointCloud(pc, 0.5, 0.01)
	vg.SegmentPlanesRegionGrowing(0.7, 25, 0.1, 1.0)
	test.ShouldEqual(vg.maxLabel, 6)
	_, err := vg.ConvertToPointCloudWithValue()
	test.That(t, err, test.ShouldBeNil)
	planes, nonPlaneCloud, err := vg.GetPlanesFromLabels()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planes), test.ShouldEqual, 6)
	test.That(t, nonPlaneCloud.Size(), test.ShouldEqual, 0)
}

func TestEstimatePlaneNormalFromPoints(t *testing.T) {
	nPoints := 1000
	points := make([]r3.Vector, 0, nPoints)
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
	points := make([]r3.Vector, 0, nPoints)
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
	plane := &voxelPlane{
		normal:    r3.Vector{0, 0, 1},
		center:    r3.Vector{},
		offset:    0,
		points:    nil,
		voxelKeys: nil,
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

func TestNNearestVoxel(t *testing.T) {
	// make the voxel grid
	voxelSize := 1.0
	pc := New()
	vox0 := NewVector(0., 0., 0.)
	test.That(t, pc.Set(vox0, nil), test.ShouldBeNil)
	vox1 := NewVector(1.1, 1.2, 1.3)
	test.That(t, pc.Set(vox1, nil), test.ShouldBeNil)
	vox2 := NewVector(0.5, 0.5, 1.8)
	test.That(t, pc.Set(vox2, nil), test.ShouldBeNil)
	vox3 := NewVector(0.3, 1.9, 0.1)
	test.That(t, pc.Set(vox3, nil), test.ShouldBeNil)
	vox4 := NewVector(1.3, 0.8, 0.5)
	test.That(t, pc.Set(vox4, nil), test.ShouldBeNil)
	vox5 := NewVector(4.5, 4.2, 3.9)
	test.That(t, pc.Set(vox5, nil), test.ShouldBeNil)
	vg := NewVoxelGridFromPointCloud(pc, voxelSize, 0.01)
	// expect 4 voxels nearest
	neighbors := vg.GetNNearestVoxels(vg.GetVoxelFromKey(VoxelCoords{0, 0, 0}), 1)
	test.That(t, len(neighbors), test.ShouldEqual, 4)
	// expect 5 voxels nearest
	neighbors = vg.GetNNearestVoxels(vg.GetVoxelFromKey(VoxelCoords{0, 0, 0}), 5)
	test.That(t, len(neighbors), test.ShouldEqual, 5)
	// expect 5 voxels nearest from the other side
	neighbors = vg.GetNNearestVoxels(vg.GetVoxelFromKey(VoxelCoords{4, 4, 3}), 5)
	test.That(t, len(neighbors), test.ShouldEqual, 5)
	// no nearest voxels
	neighbors = vg.GetNNearestVoxels(vg.GetVoxelFromKey(VoxelCoords{4, 4, 3}), 1)
	test.That(t, len(neighbors), test.ShouldEqual, 0)
}
