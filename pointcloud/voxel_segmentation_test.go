package pointcloud

import (
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

// RandomCubeSide choose a random integer between 0 and 5 that correspond to one facet of a cube.
func RandomCubeSide() int {
	//nolint: revive
	min := 0
	//nolint: revive
	max := 6
	return rand.Intn(max-min) + min
}

// GeneratePointsOnPlaneZ0 generates points on the z=0 plane.
func GeneratePointsOnPlaneZ0(nPoints int, normal r3.Vector, offset float64) PointCloud {
	pc := NewBasicPointCloud(0)
	for i := 0; i < nPoints; i++ {
		// Point in the R3 unit cube
		p := r3.Vector{rand.Float64(), rand.Float64(), 0}

		pt := NewVector(p.X, p.Y, p.Z)
		err := pc.Set(pt, nil)
		if err != nil {
			panic(err)
		}
	}
	return pc
}

// GenerateCubeTestData generate 3d points on the R^3 unit cube.
func GenerateCubeTestData(nPoints int) PointCloud {
	pc := NewBasicPointCloud(0)
	for i := 0; i < nPoints; i++ {
		// get cube side number
		s := RandomCubeSide()
		// get normal vector axis
		// if c in {0,3}, generated point will be on a plane with normal vector (1,0,0)
		// if c in {1,4}, generated point will be on a plane with normal vector (0,1,0)
		// if c in {2,5}, generated point will be on a plane with normal vector (0,0,1)
		c := int(math.Mod(float64(s), 3))
		pt := make([]float64, 3)
		pt[c] = 0
		// if side number is >=3, get side of cube at side=1
		if s > 2 {
			pt[c] = 1.0
		}
		// get other 2 point coordinates in [0,1]
		idx2 := int(math.Mod(float64(c+1), 3))
		pt[idx2] = rand.Float64()
		idx3 := int(math.Mod(float64(c+2), 3))
		pt[idx3] = rand.Float64()
		// add point to slice
		p := NewVector(pt[0], pt[1], pt[2])
		err := pc.Set(p, nil)
		if err != nil {
			panic(err)
		}
	}
	return pc
}

func TestVoxelPlaneSegmentationOnePlane(t *testing.T) {
	nPoints := 100000
	pc := GeneratePointsOnPlaneZ0(nPoints, r3.Vector{0, 0, 1}, 0.01)
	vg := NewVoxelGridFromPointCloud(pc, 0.1, 1.0)
	test.That(t, len(vg.Voxels), test.ShouldAlmostEqual, 100)
	vg.SegmentPlanesRegionGrowing(0.5, 25, 0.1, 0.05)
	pcOut, err := vg.ConvertToPointCloudWithValue()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcOut.Size(), test.ShouldBeGreaterThan, 0)
	// Labeling should find one plane
	test.That(t, vg.maxLabel, test.ShouldEqual, 1)
}

func TestVoxelPlaneSegmentationCube(t *testing.T) {
	nPoints := 10000
	pc := GenerateCubeTestData(nPoints)
	vg := NewVoxelGridFromPointCloud(pc, 0.5, 0.01)
	vg.SegmentPlanesRegionGrowing(0.7, 25, 0.1, 1.0)
	pcOut, err := vg.ConvertToPointCloudWithValue()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pcOut.Size(), test.ShouldBeGreaterThan, 0)
	// Labeling should find 6 planes
	test.That(t, vg.maxLabel, test.ShouldEqual, 6)
}

// A point in an unlabelled voxel must take the label of the nearest labelled neighbour's plane.
// Measuring against the unlabelled voxel's own plane instead yields a distance that does not vary
// with the neighbour, so the nearest is never found and the label is whichever neighbour the map
// happened to yield first.
func TestLabelNonPlanarVoxelsUsesNeighborPlanes(t *testing.T) {
	pt := r3.Vector{X: 0, Y: 0, Z: 0}
	planeAt := func(offset float64) (r3.Vector, float64) {
		// normal +Z with the given offset puts the plane |offset| away from a point on the XY origin
		return r3.Vector{X: 0, Y: 0, Z: 1}, offset
	}

	vg := &VoxelGrid{Voxels: map[VoxelCoords]*Voxel{}, voxelSize: 1, lam: 0.1}

	// The unlabelled voxel holds the point. Its own plane is deliberately far away, so relying on it
	// leaves the point below no threshold at all.
	target := NewVoxel(VoxelCoords{0, 0, 0})
	target.Points[pt] = NewBasicData()
	target.Normal, target.Offset = planeAt(-50)
	vg.Voxels[target.Key] = target

	// Two labelled neighbours: label 1 is nearer (1mm), label 2 is further (10mm).
	near := NewVoxel(VoxelCoords{1, 0, 0})
	near.Label = 1
	near.Normal, near.Offset = planeAt(-1)
	vg.Voxels[near.Key] = near

	far := NewVoxel(VoxelCoords{-1, 0, 0})
	far.Label = 2
	far.Normal, far.Offset = planeAt(-10)
	vg.Voxels[far.Key] = far

	vg.LabelNonPlanarVoxels([]VoxelCoords{target.Key}, 5.0)

	test.That(t, len(target.PointLabels), test.ShouldEqual, 1)
	test.That(t, target.PointLabels[0], test.ShouldEqual, 1)
}

// A labelled neighbour with no usable normal must not win the nearest-plane comparison just because
// Distance reports 0 for it.
func TestLabelNonPlanarVoxelsSkipsDegenerateNeighbors(t *testing.T) {
	pt := r3.Vector{X: 0, Y: 0, Z: 0}
	vg := &VoxelGrid{Voxels: map[VoxelCoords]*Voxel{}, voxelSize: 1, lam: 0.1}

	target := NewVoxel(VoxelCoords{0, 0, 0})
	target.Points[pt] = NewBasicData()
	// Far own-plane, as in the test above, so this case is not decided by map iteration order.
	target.Normal, target.Offset = r3.Vector{X: 0, Y: 0, Z: 1}, -50
	vg.Voxels[target.Key] = target

	degenerate := NewVoxel(VoxelCoords{1, 0, 0})
	degenerate.Label = 7
	degenerate.Normal = r3.Vector{} // never estimated
	vg.Voxels[degenerate.Key] = degenerate

	usable := NewVoxel(VoxelCoords{-1, 0, 0})
	usable.Label = 3
	usable.Normal, usable.Offset = r3.Vector{X: 0, Y: 0, Z: 1}, -2
	vg.Voxels[usable.Key] = usable

	vg.LabelNonPlanarVoxels([]VoxelCoords{target.Key}, 5.0)
	test.That(t, target.PointLabels[0], test.ShouldEqual, 3)
}
