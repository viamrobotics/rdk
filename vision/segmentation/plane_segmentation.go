// Package segmentation implements object segmentation algorithms.
package segmentation

import (
	"context"
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

func distance(equation [4]float64, pt pc.Vec3) float64 {
	norm := math.Sqrt(equation[0]*equation[0] + equation[1]*equation[1] + equation[2]*equation[2])
	return (equation[0]*pt.X + equation[1]*pt.Y + equation[2]*pt.Z + equation[3]) / norm
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

// SegmentPlane segments the biggest plane in the 3D Pointcloud.
// nIterations is the number of iteration for ransac
// nIter to choose? nIter = log(1-p)/log(1-(1-e)^s), where p is prob of success, e is outlier ratio, s is subset size (3 for plane).
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// This function returns a Plane struct, as well as the remaining points in a pointcloud
// It also returns the equation of the found plane: [0]x + [1]y + [2]z + [3] = 0
func SegmentPlane(ctx context.Context, cloud pc.PointCloud, nIterations int, threshold float64) (pc.Plane, pc.PointCloud, error) {
	if cloud.Size() <= 3 { // if point cloud does not have even 3 points, return original cloud with no planes
		return pc.NewEmptyPlane(), cloud, nil
	}
	r := rand.New(rand.NewSource(1))
	pts := GetPointCloudPositions(cloud)
	nPoints := cloud.Size()
	if nPoints == 0 {
		return pc.NewEmptyPlane(), pc.New(), nil
	}

	// First get all equations
	equations := make([][4]float64, nIterations)
	if err := utils.GroupWorkParallel(
		ctx,
		nIterations,
		func(numGroups int) {},
		func(groupNum, groupSize, from, to int) (utils.MemberWorkFunc, utils.GroupWorkDoneFunc) {
			for i := from; i < to; i++ {
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
				currentEquation := [4]float64{vec.X, vec.Y, vec.Z, d}
				equations[i] = currentEquation
			}
			return nil, nil
		},
	); err != nil {
		return nil, nil, err
	}

	// Then find the best equation in parallel. It ends up being faster to loop
	// by equations (iterations) and then points due to what I (erd) think is
	// memory locality exploitation.
	var bestEquation [4]float64
	type bestResult struct {
		equation [4]float64
		inliers  int
	}
	var bestResults []bestResult
	if err := utils.GroupWorkParallel(
		ctx,
		nIterations,
		func(numGroups int) {
			bestResults = make([]bestResult, numGroups)
		},
		func(groupNum, groupSize, from, to int) (utils.MemberWorkFunc, utils.GroupWorkDoneFunc) {
			bestEquation := [4]float64{}
			bestInliers := 0
			return func(memberNum int, workNum int) {
					currentInliers := 0
					currentEquation := equations[workNum]
					// store all the Points that are below a certain distance to the plane
					for _, pt := range pts {
						dist := distance(currentEquation, pt)
						if math.Abs(dist) < threshold {
							currentInliers++
						}
					}
					// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
					if currentInliers > bestInliers {
						bestEquation = currentEquation
						bestInliers = currentInliers
					}
				}, func() {
					bestResults[groupNum] = bestResult{bestEquation, bestInliers}
				}
		},
	); err != nil {
		return nil, nil, err
	}

	bestIdx := 0
	bestInliers := 0
	for i, result := range bestResults {
		if result.inliers > bestInliers {
			bestIdx = i
		}
	}
	bestEquation = bestResults[bestIdx].equation

	planeCloud := pc.NewWithPrealloc(bestInliers)
	nonPlaneCloud := pc.NewWithPrealloc(nPoints - bestInliers)
	planeCloudCenter := r3.Vector{}
	for _, pt := range pts {
		dist := distance(bestEquation, pt)
		var err error
		cpt := cloud.At(pt.X, pt.Y, pt.Z)
		if cpt == nil {
			return nil, nil, errors.Errorf("expected cloud to contain point (%v, %v, %v)", pt.X, pt.Y, pt.Z)
		}
		if math.Abs(dist) < threshold {
			planeCloudCenter = planeCloudCenter.Add(r3.Vector(pt))
			err = planeCloud.Set(cloud.At(pt.X, pt.Y, pt.Z))
		} else {
			err = nonPlaneCloud.Set(cloud.At(pt.X, pt.Y, pt.Z))
		}
		if err != nil {
			return nil, nil, errors.Errorf("error setting point (%v, %v, %v) in point cloud - %w", pt.X, pt.Y, pt.Z, err)
		}
	}

	if planeCloud.Size() != 0 {
		planeCloudCenter = planeCloudCenter.Mul(1. / float64(planeCloud.Size()))
	}

	plane := pc.NewPlaneWithCenter(planeCloud, bestEquation, pc.Vec3(planeCloudCenter))
	return plane, nonPlaneCloud, nil
}

// PlaneSegmentation is an interface used to find geometric planes in a 3D space
type PlaneSegmentation interface {
	FindPlanes(ctx context.Context) ([]pc.Plane, pc.PointCloud, error)
}

type pointCloudPlaneSegmentation struct {
	cloud       pc.PointCloud
	threshold   float64
	minPoints   int
	nIterations int
}

// NewPointCloudPlaneSegmentation initializes the plane segmentation with the necessary parameters to find the planes
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it.
// minPoints is the minimum number of points necessary to be considered a plane.
func NewPointCloudPlaneSegmentation(cloud pc.PointCloud, threshold float64, minPoints int) PlaneSegmentation {
	return &pointCloudPlaneSegmentation{cloud, threshold, minPoints, 2000}
}

// FindPlanes takes in a point cloud and outputs an array of the planes and a point cloud of the leftover points.
func (pcps *pointCloudPlaneSegmentation) FindPlanes(ctx context.Context) ([]pc.Plane, pc.PointCloud, error) {
	planes := make([]pc.Plane, 0)
	var err error
	plane, nonPlaneCloud, err := SegmentPlane(ctx, pcps.cloud, pcps.nIterations, pcps.threshold)
	if err != nil {
		return nil, nil, err
	}
	planeCloud, err := plane.PointCloud()
	if err != nil {
		return nil, nil, err
	}
	if planeCloud.Size() <= pcps.minPoints {
		return planes, pcps.cloud, nil
	}
	planes = append(planes, plane)
	lastNonPlaneCloud := nonPlaneCloud
	for {
		smallerPlane, smallerNonPlaneCloud, err := SegmentPlane(ctx, nonPlaneCloud, pcps.nIterations, pcps.threshold)
		if err != nil {
			return nil, nil, err
		}
		planeCloud, err := smallerPlane.PointCloud()
		if err != nil {
			return nil, nil, err
		}
		if planeCloud.Size() <= pcps.minPoints {
			// this cloud is not valid so revert to last
			nonPlaneCloud = lastNonPlaneCloud
			if err != nil {
				return nil, nil, err
			}
			break
		} else {
			nonPlaneCloud = smallerNonPlaneCloud
		}
		planes = append(planes, smallerPlane)
	}
	return planes, nonPlaneCloud, nil
}

// VoxelGridPlaneConfig contains the parameters needed to create a Plane from a VoxelGrid
type VoxelGridPlaneConfig struct {
	weightThresh   float64
	angleThresh    float64 // in degrees
	cosineThresh   float64
	distanceThresh float64
}

type voxelGridPlaneSegmentation struct {
	*pc.VoxelGrid
	config VoxelGridPlaneConfig
}

// NewVoxelGridPlaneSegmentation initializes the necessary parameters needed to do plane segmentation on a voxel grid.
func NewVoxelGridPlaneSegmentation(vg *pc.VoxelGrid, config VoxelGridPlaneConfig) PlaneSegmentation {
	return &voxelGridPlaneSegmentation{vg, config}
}

// FindPlanes takes in a point cloud and outputs an array of the planes and a point cloud of the leftover points.
func (vgps *voxelGridPlaneSegmentation) FindPlanes(ctx context.Context) ([]pc.Plane, pc.PointCloud, error) {
	vgps.SegmentPlanesRegionGrowing(vgps.config.weightThresh, vgps.config.angleThresh, vgps.config.cosineThresh, vgps.config.distanceThresh)
	planes, nonPlaneCloud, err := vgps.GetPlanesFromLabels()
	if err != nil {
		return nil, nil, err
	}
	return planes, nonPlaneCloud, nil
}

// SplitPointCloudByPlane divides the point cloud in two point clouds, given the equation of a plane.
// one point cloud will have all the points above the plane and the other with all the points below the plane.
// Points exactly on the plane are not included!
func SplitPointCloudByPlane(cloud pc.PointCloud, plane pc.Plane) (pc.PointCloud, pc.PointCloud, error) {
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
func ThresholdPointCloudByPlane(cloud pc.PointCloud, plane pc.Plane, threshold float64) (pc.PointCloud, error) {
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
