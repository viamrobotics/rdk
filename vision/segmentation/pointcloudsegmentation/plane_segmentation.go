package pointcloudsegmentation

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	pc "go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
)

// Function to sample a random integer within a range given by [min, max]
func SampleRandomIntRange(min, max int) int {
	return rand.Intn(max-min+1) + min
}

// Extract the positions of the points from the pointcloud into an r3 slice
func GetPointCloudPositions(cloud *pc.PointCloud) []r3.Vector {
	keys := make([]r3.Vector, 0, cloud.Size())
	cloud.Iterate(func(pt pc.Point) bool {
		keys = append(keys, r3.Vector(pt.Position()))
		return true
	})
	return keys
}

// Return a pointcloud that is a subset of the given pointcloud using a map of point positions
func GetSubsetPointCloud(cloud *pc.PointCloud, inPlane map[r3.Vector]bool) (*pc.PointCloud, error) {
	subCloud := pc.New()
	var err error
	cloud.Iterate(func(pt pc.Point) bool {
		if _, ok := inPlane[r3.Vector(pt.Position())]; ok {
			err = subCloud.Set(pt)
			if err != nil {
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", pt.Position().X, pt.Position().Y, pt.Position().Z, err)
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return subCloud, nil
}

// Function to segment the biggest plane in the 3D Pointcloud
// nIterations is the number of iteration for ransac
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// pixel2meter is the conversion factor from the depth value to its value in meters
// This function returns a pointcloud that only contains the points in the found plane
func SegmentPlane(cloud *pc.PointCloud, nIterations int, threshold, pixel2meter float64) (*pc.PointCloud, []float64, error) {
	pts := GetPointCloudPositions(cloud)
	nPoints := cloud.Size()
	bestEquation := make([]float64, 4)
	currentEquation := make([]float64, 4)
	bestInliers := make(map[r3.Vector]bool)

	for i := 0; i < nIterations; i++ {
		// sample 3 Points from the slice of 3D Points
		n1, n2, n3 := SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1)
		p1, p2, p3 := pts[n1], pts[n2], pts[n3]

		// get 2 vectors that are going to define the plane
		v1 := p2.Sub(p1)
		v2 := p3.Sub(p1)
		// cross product to get the normal unit vector to the plane (v1, v2)
		cross := v1.Cross(v2)
		vec := cross.Normalize()
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		// to find d, we just need to pick a point and deduce d from the plane equation (vec orth to p1, p2, p3)
		d := -vec.Dot(p2)
		// current plane equation
		currentEquation[0], currentEquation[1], currentEquation[2], currentEquation[3] = vec.X, vec.Y, vec.Z, d

		// compute distance to plane of each point in the cloud
		currentInliers := make(map[r3.Vector]bool)
		// store all the Points that are below a certain distance to the plane
		for _, pt := range pts {
			dist := (currentEquation[0]*pt.X + currentEquation[1]*pt.Y + currentEquation[2]*pt.Z + currentEquation[3]) / vec.Norm()
			if math.Abs(dist) < threshold {
				currentInliers[pt] = true
			}
		}
		// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
		if len(currentInliers) > len(bestInliers) {
			bestEquation = currentEquation
			bestInliers = currentInliers
		}
	}
	bestCloud, err := GetSubsetPointCloud(cloud, bestInliers)
	if err != nil {
		return nil, nil, err
	}
	return bestCloud, bestEquation, nil
}

// utils for 3D float Points transforms
// Get plane 2D mask obtained from depth data in RGB image coordinates
func GetPlaneMaskRGBPointCloud(depthImage *rimage.DepthMap, coordinates *pc.PointCloud) image.Image {
	h, w := depthImage.Height(), depthImage.Width()
	upLeft := image.Point{0, 0}
	lowRight := image.Point{w, h}

	maskPlane := image.NewGray(image.Rectangle{upLeft, lowRight})

	// Colors are defined by Red, Green, Blue, Alpha uint8 values.
	black := color.Gray{0}

	// Set color for each pixel.
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			maskPlane.Set(y, x, black)
		}
	}
	white := color.Gray{255}
	coordinates.Iterate(func(pt pc.Point) bool {
		j, i := pt.Position().X, pt.Position().Y
		maskPlane.Set(int(j), int(i), white)
		return true
	})
	return maskPlane
}
