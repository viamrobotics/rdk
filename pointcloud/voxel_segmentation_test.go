package pointcloud

import (
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
)

// RandomCubeSide choose a random integer between 0 and 5 that correspond to one facet of a cube
func RandomCubeSide() int {
	min := 0
	max := 6
	return rand.Intn(max-min) + min
}

// GenerateTestData generate 3d points on the R^3 unit cube
func GenerateTestData(nPoints int) []r3.Vector {
	points := make([]r3.Vector, 0)
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
		points = append(points, r3.Vector{pt[0], pt[1], pt[2]})
	}
	return points
}
