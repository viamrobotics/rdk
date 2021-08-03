// Package segmentation implements object segmentation algorithms.
package segmentation

import (
	"image"
	"math"
	"math/rand"
	"sort"

	"github.com/go-errors/errors"

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
			err = errors.Errorf("error setting point (%v, %v, %v) in point cloud - %w", pos.X, pos.Y, pos.Z, err)
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

// Plane defines a planar object in a point cloud
type Plane struct {
	pointcloud pc.PointCloud
	equation   []float64
}

// NewEmptyPlane initializes an empty plane object
func NewEmptyPlane() *Plane {
	return &Plane{pc.New(), []float64{0, 0, 0, 0, 0}}
}

// PointCloud returns the underlying point cloud of the plane
func (p *Plane) PointCloud() pc.PointCloud {
	return p.pointcloud
}

// Equation returns the plane equation [0]x + [1]y + [2]z + [3] = 0. [4] is the 2-norm of the normal vector.
func (p *Plane) Equation() []float64 {
	return p.equation
}

// Distance calculates the distance from the plane to the input point
func (p *Plane) Distance(point pc.Vec3) float64 {
	return distance(p.equation, point)
}

// SegmentPlane segments the biggest plane in the 3D Pointcloud.
// nIterations is the number of iteration for ransac
// nIter to choose? nIter = log(1-p)/log(1-(1-e)^s), where p is prob of success, e is outlier ratio, s is subset size (3 for plane).
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// This function returns a Plane struct, as well as the remaining points in a pointcloud
// It also returns the equation of the found plane: [0]x + [1]y + [2]z + [3] = 0
func SegmentPlane(cloud pc.PointCloud, nIterations int, threshold float64) (*Plane, pc.PointCloud, error) {
	if cloud.Size() <= 3 { // if point cloud does not have even 3 points, return original cloud with no planes
		return NewEmptyPlane(), cloud, nil
	}
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
		return nil, nil, err
	}
	return &Plane{planeCloud, bestEquation}, nonPlaneCloud, nil
}

// FindPlanesInPointCloud takes in a point cloud and outputs an array of the planes and a point cloud of
// the leftover points.
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it.
// minPoints is the minimum number of points necessary to be considered a plane.
func FindPlanesInPointCloud(cloud pc.PointCloud, threshold float64, minPoints int) ([]*Plane, pc.PointCloud, error) {
	planes := make([]*Plane, 0)
	var err error
	plane, nonPlaneCloud, err := SegmentPlane(cloud, 2000, threshold)
	if err != nil {
		return nil, nil, err
	}
	if plane.PointCloud().Size() <= minPoints {
		return planes, cloud, nil
	}
	planes = append(planes, plane)
	for {
		plane, nonPlaneCloud, err = SegmentPlane(nonPlaneCloud, 2000, threshold)
		if err != nil {
			return nil, nil, err
		}
		if plane.PointCloud().Size() <= minPoints {
			// add the failed planeCloud back into the nonPlaneCloud
			plane.PointCloud().Iterate(func(pt pc.Point) bool {
				err = nonPlaneCloud.Set(pt)
				return err == nil
			})
			if err != nil {
				return nil, nil, err
			}
			break
		}
		planes = append(planes, plane)
	}
	return planes, nonPlaneCloud, nil
}

// SplitPointCloudByPlane divides the point cloud in two point clouds, given the equation of a plane.
// one point cloud will have all the points above the plane and the other with all the points below the plane.
// Points exactly on the plane are not included!
func SplitPointCloudByPlane(cloud pc.PointCloud, plane *Plane) (pc.PointCloud, pc.PointCloud, error) {
	aboveCloud, belowCloud := pc.New(), pc.New()
	var err error
	cloud.Iterate(func(pt pc.Point) bool {
		dist := plane.Distance(pt.Position())
		if plane.Equation()[2] > 0.0 {
			dist = -dist
		}
		if dist > 0.0 {
			err = aboveCloud.Set(pt)
		} else if dist < 0.0 {
			err = belowCloud.Set(pt)
		}
		return err == nil
	})
	if err != nil {
		return nil, nil, err
	}
	return aboveCloud, belowCloud, nil
}

// ThresholdPointCloudByPlane returns a pointcloud with the points less than or equal to a given distance from a given plane.
func ThresholdPointCloudByPlane(cloud pc.PointCloud, plane *Plane, threshold float64) (pc.PointCloud, error) {
	thresholdCloud := pc.New()
	var err error
	cloud.Iterate(func(pt pc.Point) bool {
		dist := plane.Distance(pt.Position())
		if math.Abs(dist) <= threshold {
			err = thresholdCloud.Set(pt)
		}
		return err == nil
	})
	if err != nil {
		return nil, err
	}
	return thresholdCloud, nil
}

// PointCloudSegmentsToMask takes in an instrinsic camera matrix and a slice of pointclouds and projects
// each pointcloud down to an image.
func PointCloudSegmentsToMask(params transform.PinholeCameraIntrinsics, segments []pc.PointCloud) (*SegmentedImage, error) {
	img := newSegmentedImage(rimage.NewImage(params.Width, params.Height))
	visitedPoints := make(map[pc.Vec3]bool)
	var err error
	for i, cloud := range segments {
		cloud.Iterate(func(pt pc.Point) bool {
			pos := pt.Position()
			if seen := visitedPoints[pos]; seen {
				err = errors.Errorf("point clouds in array must be distinct, have already seen point (%v,%v,%v)", pos.X, pos.Y, pos.Z)
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
