// Package segmentation implements object segmentation algorithms.
package segmentation

import (
	"errors"
	"fmt"
	"image"
	"math"
	"math/rand"
	"sort"

	pc "go.viam.com/core/pointcloud"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
)

var sortPositions bool

// GetPointCloudPositions extracts the positions of the points from the pointcloud into a Vec3 slice.
func GetPointCloudPositions(cloud pc.PointCloud) []pc.Vec3 {
	positions := make([]pc.Vec3, 0, cloud.Size())
	cloud.Iterate(func(pt pc.Point) bool {
		positions = append(positions, pt.Position())
		return true
	})
	if sortPositions {
		sort.Sort(pc.Vec3s(positions))
	}
	return positions
}

func distance(equation []float64, pt pc.Vec3) float64 {
	return (equation[0]*pt.X + equation[1]*pt.Y + equation[2]*pt.Z + equation[3]) / equation[4]
}

// pointCloudSplit return two point clouds, one with points found in a map of point positions, and the other with those not in the map.
func pointCloudSplit(cloud pc.PointCloud, inMap map[pc.Vec3]bool) (pc.PointCloud, pc.PointCloud, error) {
	mapCloud := pc.New()
	nonMapCloud := pc.New()
	var err error
	seen := make(map[pc.Vec3]bool)
	cloud.Iterate(func(pt pc.Point) bool {
		if _, ok := inMap[pt.Position()]; ok {
			seen[pt.Position()] = true
			err = mapCloud.Set(pt)
		} else {
			err = nonMapCloud.Set(pt)
		}
		if err != nil {
			pos := pt.Position()
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %w", pos.X, pos.Y, pos.Z, err)
			return false
		}
		return true
	})
	if err != nil {
		return nil, nil, err
	}
	if len(seen) != len(inMap) {
		err = errors.New("map of points contains invalid points not found in the point cloud")
		return nil, nil, err
	}
	return mapCloud, nonMapCloud, nil
}

// SegmentPlane segments the biggest plane in the 3D Pointcloud.
// nIterations is the number of iteration for ransac
// nIter to choose? nIter = log(1-p)/log(1-(1-e)^s), where p is prob of success, e is outlier ratio, s is subset size (3 for plane).
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// This function returns 2 pointclouds, the pointcloud of the plane and one without the plane
// It also returns the equation of the found plane: [0]x + [1]y + [2]z + [3] = 0
func SegmentPlane(cloud pc.PointCloud, nIterations int, threshold float64) (pc.PointCloud, pc.PointCloud, []float64, error) {
	r := rand.New(rand.NewSource(1))
	pts := GetPointCloudPositions(cloud)
	nPoints := cloud.Size()

	bestEquation := make([]float64, 4)
	bestInliers := 0

	for i := 0; i < nIterations; i++ {
		// sample 3 Points from the slice of 3D Points
		n1, n2, n3 := utils.SampleRandomIntRange(1, nPoints-1, r), utils.SampleRandomIntRange(1, nPoints-1, r), utils.SampleRandomIntRange(1, nPoints-1, r)
		p1, p2, p3 := r3.Vector(pts[n1]), r3.Vector(pts[n2]), r3.Vector(pts[n3])

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
		currentEquation := []float64{vec.X, vec.Y, vec.Z, d, vec.Norm()}

		// compute distance to plane of each point in the cloud
		//currentInliers := make([]pc.Vec3, len(bestInliers))
		currentInliers := 0

		// store all the Points that are below a certain distance to the plane
		for _, pt := range pts {
			//dist := (currentEquation[0]*pt.X + currentEquation[1]*pt.Y + currentEquation[2]*pt.Z + currentEquation[3]) / vec.Norm()
			dist := distance(currentEquation, pt)
			if math.Abs(dist) < threshold {
				currentInliers++
				//currentInliers = append(currentInliers, pt)
			}
		}
		// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
		if currentInliers > bestInliers {
			bestEquation = currentEquation
			bestInliers = currentInliers
		}
	}

	bestInliersMap := make(map[pc.Vec3]bool)
	for _, pt := range pts {
		dist := distance(bestEquation, pt)
		if math.Abs(dist) < threshold {
			bestInliersMap[pt] = true
		}
	}

	planeCloud, nonPlaneCloud, err := pointCloudSplit(cloud, bestInliersMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return planeCloud, nonPlaneCloud, bestEquation, nil
}

// GetPlanesInPointCloud takes in a point cloud and outputs an array of the plane pointclouds and a point cloud of
// the leftover points.
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it.
// minPoints is the minimum number of points necessary to be considered a plane.
func GetPlanesInPointCloud(cloud pc.PointCloud, threshold float64, minPoints int) ([]pc.PointCloud, pc.PointCloud, error) {
	planes := make([]pc.PointCloud, 0)
	var err error
	planeCloud, nonPlaneCloud, _, err := SegmentPlane(cloud, 2000, threshold)
	if err != nil {
		return nil, nil, err
	}
	if planeCloud.Size() <= minPoints {
		return planes, cloud, nil
	}
	planes = append(planes, planeCloud)
	for {
		planeCloud, nonPlaneCloud, _, err = SegmentPlane(nonPlaneCloud, 2000, threshold)
		if err != nil {
			return nil, nil, err
		}
		if planeCloud.Size() <= minPoints {
			// add the failed planeCloud back into the nonPlaneCloud
			planeCloud.Iterate(func(pt pc.Point) bool {
				err = nonPlaneCloud.Set(pt)
				return err == nil
			})
			if err != nil {
				return nil, nil, err
			}
			break
		}
		planes = append(planes, planeCloud)
	}
	return planes, nonPlaneCloud, nil
}

// PointCloudSegmentsToMask takes in an instrinsic camera matrix and a slice of pointclouds and projects
// each pointcloud down to an image.
func pointCloudSegmentsToMask(params transform.PinholeCameraIntrinsics, segments []pc.PointCloud) (*SegmentedImage, error) {
	img := newSegmentedImage(rimage.NewImage(params.Width, params.Height))
	visitedPoints := make(map[pc.Vec3]bool)
	var err error
	for i, cloud := range segments {
		cloud.Iterate(func(pt pc.Point) bool {
			pos := pt.Position()
			if seen := visitedPoints[pos]; seen {
				err = fmt.Errorf("point clouds in array must be distinct, have already seen point (%v,%v,%v)", pos.X, pos.Y, pos.Z)
				return false
			}
			visitedPoints[pos] = true
			px, py := params.PointToPixel(pos.X, pos.Y, pos.Z)
			px, py = math.Round(px), math.Round(py)
			x, y := int(px), int(py)
			if x >= 0 && x < params.Width && y >= 0 && y < params.Height {
				img.set(image.Point{x, y}, i+1)
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	img.createPalette()
	return img, nil
}
