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
	min := 0
	max := 6
	return rand.Intn(max-min) + min
}

// GeneratePointsOnPlaneZ0 generates points on the z=0 plane.
func GeneratePointsOnPlaneZ0(nPoints int, normal r3.Vector, offset float64) PointCloud {
	pc := New()
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
	pc := New()
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
