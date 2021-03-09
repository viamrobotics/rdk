package point_cloud_segmentation

import (
	"fmt"
	"github.com/golang/geo/r3"
	"go.viam.com/robotcore/rimage"
	"image"
	"math"
	"math/rand"
)

// Function to detect a plane in a 3D point cloud;
// Returns a slice of PointCloud.keys of the 3D points that belong to found plane
// And the parameters of the plane equation as a slice of float64 (useful for determining points above plane)
func SegmentPlane(pointCloud *PointCloudFloat, nIterations int, threshold, pixel2meter float64) ([]r3.Vector, []float64) {

	var keys []r3.Vector
	pointCloud.Iterate(func(p PointFloat) bool {
		v := p.Position()
		//, v.Y, v.Z
		keys = append(keys, r3.Vector{float64(v.X), float64(v.Y), float64(v.Z)})
		return true
	})
	nPoints := len(keys)
	fmt.Printf("NPoints %d \n", nPoints)
	bestEquation := make([]float64, 4)
	currentEquation := make([]float64, 4)
	var bestInliers []r3.Vector

	var currentInliers []r3.Vector
	var p1, p2, p3, v1, v2, cross, vec r3.Vector
	var k1, k2, k3 int
	var d, dist float64

	for i := 0; i < nIterations; i++ {
		// bar increment
		// sample 3 points
		k1, k2, k3 = rand.Intn(nPoints), rand.Intn(nPoints), rand.Intn(nPoints)

		key1, key2, key3 := keys[k1], keys[k2], keys[k3]
		p1 = r3.Vector{key1.X, key1.Y, key1.Z}
		p2 = r3.Vector{key2.X, key2.Y, key2.Z}
		p3 = r3.Vector{key3.X, key3.Y, key3.Z}

		// get 2 vectors

		v1 = p2.Sub(p1)
		v2 = p3.Sub(p1)
		// cross product
		cross = v1.Cross(v2)
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		vec = cross.Normalize()
		// in order to find d, we just need to pick a point and deduce d from the plane equation
		d = - vec.Dot(p1)
		// current plane equation
		currentEquation[0], currentEquation[1], currentEquation[2], currentEquation[3] = vec.X, vec.Y, vec.Z, d

		// compute distance to plane of each point in the cloud
		currentInliers = nil
		for _, pt := range keys {
			dist = (currentEquation[0]*float64(pt.X) + currentEquation[1]*float64(pt.Y) + currentEquation[2]*float64(pt.Z) + currentEquation[3]) / vec.Norm()
			if math.Abs(dist) < threshold {
				//fmt.Println(dist)
				currentInliers = append(currentInliers, pt)
			}
		}
		if len(currentInliers) > len(bestInliers) {
			bestEquation = currentEquation
			bestInliers = nil
			bestInliers = currentInliers
		}
	}
	// Output as slice of PointCloud.Vec3
	var bestInliersPointCloud []r3.Vector
	for _, pt := range bestInliers {
		bestInliersPointCloud = append(bestInliersPointCloud, pt)
	}
	return bestInliersPointCloud, bestEquation
}

// Function to create mask
func CreateMaskPlaneRemoval(pts []r3.Vector, planeInliers []int) []float64 {
	nPoints := len(pts)
	// create new mask filled with ones
	mask := make([]float64, nPoints)
	for j := range mask {
		mask[j] = 1.
	}
	// fill points corresponding to plane to 0
	for _, idx := range planeInliers {
		mask[idx] = 0.
	}
	return mask
}

// Function that creates a point cloud from a depth image with the intrinsics from the depth sensor camera
// For intel L515 - 640 x 480 (multiply each parameter by 2 for 1280 x 960 resolution) for depth data that was not
// aligned with RGB sensor
// cx, cy (Principal PointFloat)         : 338.734, 248.449
// fx, fy (Focal Length)            : 459.336, 459.691
// For RGB sensor (depth image aligned with RGB 1280 x 720 resolution)
// cx, cy (Principal PointFloat)         : 648.934, 367.736
// fx, fy (Focal Length)            : 900.538, 900.818
//Distortion Coefficients : [0.158701,-0.485405,-0.00143327,-0.000705919,0.435342]
func DepthMapToPointCloud(depthImage *rimage.DepthMap, pixel2meter, cx, cy, fx, fy float64) *PointCloudFloat {
	// create new point cloud
	pc_ := New()
	fmt.Println(depthImage.MinMax())
	// output slice
	points := make(map[r3.Vector]PointFloat, depthImage.Width()*depthImage.Height())
	// go through depth map pixels and get 3D points
	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			// get z distance to meter for unit uniformity
			dm := float64(d) * pixel2meter
			// get x and y over z
			xOverZ := (cx - float64(x)) / fx
			yOverZ := (cy - float64(y)) / fy
			// get z
			z := dm / math.Sqrt(1.0 + xOverZ*xOverZ + yOverZ*yOverZ)
			// get x and y
			x_ := xOverZ * z
			y_ := yOverZ * z
			if x==0 && y==0 {
				fmt.Println(x_, y_, z)
			}
			// Get point in PointCloud format
			pt := NewPointFloat(x_, y_, z)
			pc_.Set(pt)
			points[pt.Position()] = pt

		}

	}
	return pc_
}
