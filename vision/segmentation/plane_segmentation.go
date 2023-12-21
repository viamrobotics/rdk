// Package segmentation implements object segmentation algorithms.
package segmentation

import (
	"context"
	"image"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// Setting a global here is dangerous and doesn't work in parallel.
// This is only ever set for testing, and tests need to be careful to revert it to false afterwards.
var sortPositions bool

// GetPointCloudPositions extracts the positions of the points from the pointcloud into a Vec3 slice.
func GetPointCloudPositions(cloud pc.PointCloud) ([]r3.Vector, []pc.Data) {
	positions := make(pc.Vectors, 0, cloud.Size())
	data := make([]pc.Data, 0, cloud.Size())
	cloud.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		positions = append(positions, pt)
		data = append(data, d)
		return true
	})
	if sortPositions {
		sort.Sort(positions)
	}
	return positions, data
}

func distance(equation [4]float64, pt r3.Vector) float64 {
	norm := math.Sqrt(equation[0]*equation[0] + equation[1]*equation[1] + equation[2]*equation[2])
	return (equation[0]*pt.X + equation[1]*pt.Y + equation[2]*pt.Z + equation[3]) / norm
}

// pointCloudSplit return two point clouds, one with points found in a map of point positions, and the other with those not in the map.
func pointCloudSplit(cloud pc.PointCloud, inMap map[r3.Vector]bool) (pc.PointCloud, pc.PointCloud, error) {
	mapCloud := pc.New()
	nonMapCloud := pc.New()
	var err error
	seen := make(map[r3.Vector]bool)
	cloud.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		if _, ok := inMap[pt]; ok {
			seen[pt] = true
			err = mapCloud.Set(pt, d)
		} else {
			err = nonMapCloud.Set(pt, d)
		}
		if err != nil {
			err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", pt.X, pt.Y, pt.Z)
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

// SegmentPlaneWRTGround segments the biggest 'ground' plane in the 3D Pointcloud.
// nIterations is the number of iteration for ransac
// nIter to choose? nIter = log(1-p)/log(1-(1-e)^s), where p is prob of success, e is outlier ratio, s is subset size (3 for plane).
// dstThreshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// This function returns a Plane struct, as well as the remaining points in a pointcloud
// It also returns the equation of the found plane: [0]x + [1]y + [2]z + [3] = 0.
// angleThrehold is the maximum acceptable angle between the groundVec and angle of the plane,
// if the plane is at a larger angle than maxAngle, it will not be considered for segmentation, if set to 0 then not considered
// normalVec is the normal vector of the plane representing the ground.
func SegmentPlaneWRTGround(ctx context.Context, cloud pc.PointCloud, nIterations int, angleThreshold,
	dstThreshold float64, normalVec r3.Vector,
) (pc.Plane, pc.PointCloud, error) {
	if cloud.Size() <= 3 { // if point cloud does not have even 3 points, return original cloud with no planes
		return pc.NewEmptyPlane(), cloud, nil
	}
	//nolint:gosec
	r := rand.New(rand.NewSource(1))
	pts, data := GetPointCloudPositions(cloud)
	nPoints := cloud.Size()

	// First get all equations
	equations := make([][4]float64, 0, nIterations)
	for i := 0; i < nIterations; i++ {
		// sample 3 Points from the slice of 3D Points
		n1, n2, n3 := utils.SampleRandomIntRange(1, nPoints-1, r),
			utils.SampleRandomIntRange(1, nPoints-1, r),
			utils.SampleRandomIntRange(1, nPoints-1, r)
		p1, p2, p3 := pts[n1], pts[n2], pts[n3]

		// get 2 vectors that are going to define the plane
		v1 := p2.Sub(p1)
		v2 := p3.Sub(p1)
		// cross product to get the normal unit vector to the plane (v1, v2)
		cross := v1.Cross(v2)
		planeVec := cross.Normalize()
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		// to find d, we just need to pick a point and deduce d from the plane equation (vec orth to p1, p2, p3)
		d := -planeVec.Dot(p2)

		currentEquation := [4]float64{planeVec.X, planeVec.Y, planeVec.Z, d}

		if angleThreshold != 0 {
			if math.Acos(normalVec.Dot(planeVec)) <= angleThreshold*math.Pi/180.0 {
				equations = append(equations, currentEquation)
			}
		} else {
			equations = append(equations, currentEquation)
		}
	}
	return findBestEq(ctx, cloud, len(equations), equations, pts, data, dstThreshold)
}

// SegmentPlane segments the biggest plane in the 3D Pointcloud.
// nIterations is the number of iteration for ransac
// nIter to choose? nIter = log(1-p)/log(1-(1-e)^s), where p is prob of success, e is outlier ratio, s is subset size (3 for plane).
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// This function returns a Plane struct, as well as the remaining points in a pointcloud
// It also returns the equation of the found plane: [0]x + [1]y + [2]z + [3] = 0.
func SegmentPlane(ctx context.Context, cloud pc.PointCloud, nIterations int, threshold float64) (pc.Plane, pc.PointCloud, error) {
	if cloud.Size() <= 3 { // if point cloud does not have even 3 points, return original cloud with no planes
		return pc.NewEmptyPlane(), cloud, nil
	}
	//nolint:gosec
	r := rand.New(rand.NewSource(1))
	pts, data := GetPointCloudPositions(cloud)
	nPoints := cloud.Size()

	// First get all equations
	equations := make([][4]float64, 0, nIterations)
	for i := 0; i < nIterations; i++ {
		// sample 3 Points from the slice of 3D Points
		n1, n2, n3 := utils.SampleRandomIntRange(1, nPoints-1, r),
			utils.SampleRandomIntRange(1, nPoints-1, r),
			utils.SampleRandomIntRange(1, nPoints-1, r)
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
		currentEquation := [4]float64{vec.X, vec.Y, vec.Z, d}
		equations = append(equations, currentEquation)
	}

	return findBestEq(ctx, cloud, nIterations, equations, pts, data, threshold)
}

func findBestEq(ctx context.Context, cloud pc.PointCloud, nIterations int, equations [][4]float64,
	pts []r3.Vector, data []pc.Data, threshold float64,
) (pc.Plane, pc.PointCloud, error) {
	// Then find the best equation in parallel. It ends up being faster to loop
	// by equations (iterations) and then points due to what I (erd) think is
	// memory locality exploitation.
	var bestEquation [4]float64
	type bestResult struct {
		equation [4]float64
		inliers  int
	}
	var bestResults []bestResult
	var bestResultsMu sync.Mutex
	if err := utils.GroupWorkParallel(
		ctx,
		nIterations,
		func(numGroups int) {
			bestResults = make([]bestResult, numGroups)
		},
		func(groupNum, groupSize, from, to int) (utils.MemberWorkFunc, utils.GroupWorkDoneFunc) {
			var groupMu sync.Mutex
			bestEquation := [4]float64{}
			bestInliers := 0

			return func(memberNum, workNum int) {
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
					groupMu.Lock()
					defer groupMu.Unlock()
					if currentInliers > bestInliers {
						bestEquation = currentEquation
						bestInliers = currentInliers
					}
				}, func() {
					bestResultsMu.Lock()
					defer bestResultsMu.Unlock()
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

	nPoints := cloud.Size()

	planeCloud := pc.NewWithPrealloc(bestInliers)
	nonPlaneCloud := pc.NewWithPrealloc(nPoints - bestInliers)
	planeCloudCenter := r3.Vector{}
	for i, pt := range pts {
		dist := distance(bestEquation, pt)
		var err error
		if math.Abs(dist) < threshold {
			planeCloudCenter = planeCloudCenter.Add(pt)
			err = planeCloud.Set(pt, data[i])
		} else {
			err = nonPlaneCloud.Set(pt, data[i])
		}
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", pt.X, pt.Y, pt.Z)
		}
	}

	if planeCloud.Size() != 0 {
		planeCloudCenter = planeCloudCenter.Mul(1. / float64(planeCloud.Size()))
	}

	plane := pc.NewPlaneWithCenter(planeCloud, bestEquation, planeCloudCenter)
	return plane, nonPlaneCloud, nil
}

// PlaneSegmentation is an interface used to find geometric planes in a 3D space.
type PlaneSegmentation interface {
	FindPlanes(ctx context.Context) ([]pc.Plane, pc.PointCloud, error)
	FindGroundPlane(ctx context.Context) (pc.Plane, pc.PointCloud, error)
}

type pointCloudPlaneSegmentation struct {
	cloud             pc.PointCloud
	distanceThreshold float64
	minPoints         int
	nIterations       int
	angleThreshold    float64
	normalVec         r3.Vector
}

// NewPointCloudPlaneSegmentation initializes the plane segmentation with the necessary parameters to find the planes
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it.
// minPoints is the minimum number of points necessary to be considered a plane.
func NewPointCloudPlaneSegmentation(cloud pc.PointCloud, threshold float64, minPoints int) PlaneSegmentation {
	return &pointCloudPlaneSegmentation{
		cloud:             cloud,
		distanceThreshold: threshold,
		minPoints:         minPoints,
		nIterations:       2000,
		angleThreshold:    0,
		normalVec:         r3.Vector{X: 0, Y: 0, Z: 1},
	}
}

// NewPointCloudGroundPlaneSegmentation initializes the plane segmentation with the necessary parameters to find
// ground like planes, meaning they are less than angleThreshold away from the plane corresponding to normaLVec
// distanceThreshold is the float64 value for the maximum allowed distance to the found plane for a
// point to belong to it.
// minPoints is the minimum number of points necessary to be considered a plane.
func NewPointCloudGroundPlaneSegmentation(cloud pc.PointCloud, distanceThreshold float64, minPoints int,
	angleThreshold float64, normalVec r3.Vector,
) PlaneSegmentation {
	return &pointCloudPlaneSegmentation{cloud, distanceThreshold, minPoints, 2000, angleThreshold, normalVec}
}

// FindPlanes takes in a point cloud and outputs an array of the planes and a point cloud of the leftover points.
func (pcps *pointCloudPlaneSegmentation) FindPlanes(ctx context.Context) ([]pc.Plane, pc.PointCloud, error) {
	planes := make([]pc.Plane, 0)
	var err error
	plane, nonPlaneCloud, err := SegmentPlane(ctx, pcps.cloud, pcps.nIterations, pcps.distanceThreshold)
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
	var lastNonPlaneCloud pc.PointCloud
	for {
		lastNonPlaneCloud = nonPlaneCloud
		smallerPlane, smallerNonPlaneCloud, err := SegmentPlane(ctx, nonPlaneCloud, pcps.nIterations, pcps.distanceThreshold)
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
			break
		} else {
			nonPlaneCloud = smallerNonPlaneCloud
		}
		planes = append(planes, smallerPlane)
	}
	return planes, nonPlaneCloud, nil
}

// FindGroundPlane takes in a point cloud and outputs an array of a ground like plane and a point cloud of the leftover points.
func (pcps *pointCloudPlaneSegmentation) FindGroundPlane(ctx context.Context) (pc.Plane, pc.PointCloud, error) {
	var err error
	plane, nonPlaneCloud, err := SegmentPlaneWRTGround(ctx, pcps.cloud, pcps.nIterations,
		pcps.angleThreshold, pcps.distanceThreshold, pcps.normalVec)
	if err != nil {
		return nil, nil, err
	}
	planeCloud, err := plane.PointCloud()
	if err != nil {
		return nil, nil, err
	}
	if planeCloud.Size() <= pcps.minPoints {
		return nil, pcps.cloud, nil
	}
	return plane, nonPlaneCloud, nil
}

// VoxelGridPlaneConfig contains the parameters needed to create a Plane from a VoxelGrid.
type VoxelGridPlaneConfig struct {
	WeightThresh   float64 `json:"weight_threshold"`
	AngleThresh    float64 `json:"angle_threshold_degs"`
	CosineThresh   float64 `json:"cosine_threshold"` // between -1 and 1, the value after evaluating Cosine(theta)
	DistanceThresh float64 `json:"distance_threshold_mm"`
}

// CheckValid checks to see in the inputs values are valid.
func (vgpc *VoxelGridPlaneConfig) CheckValid() error {
	if vgpc.WeightThresh < 0 {
		return errors.Errorf("weight_threshold cannot be less than 0, got %v", vgpc.WeightThresh)
	}
	if vgpc.AngleThresh < -360 || vgpc.AngleThresh > 360 {
		return errors.Errorf("angle_threshold must be in degrees, between -360 and 360, got %v", vgpc.AngleThresh)
	}
	if vgpc.CosineThresh < -1 || vgpc.CosineThresh > 1 {
		return errors.Errorf("cosine_threshold must be between -1 and 1, got %v", vgpc.CosineThresh)
	}
	if vgpc.DistanceThresh < 0 {
		return errors.Errorf("distance_threshold cannot be less than 0, got %v", vgpc.DistanceThresh)
	}
	return nil
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
	vgps.SegmentPlanesRegionGrowing(vgps.config.WeightThresh, vgps.config.AngleThresh, vgps.config.CosineThresh, vgps.config.DistanceThresh)
	planes, nonPlaneCloud, err := vgps.GetPlanesFromLabels()
	if err != nil {
		return nil, nil, err
	}
	return planes, nonPlaneCloud, nil
}

// FindGroundPlane is yet to be implemented.
func (vgps *voxelGridPlaneSegmentation) FindGroundPlane(ctx context.Context) (pc.Plane, pc.PointCloud, error) {
	return nil, nil, errors.New("function not yet implemented")
}

// SplitPointCloudByPlane divides the point cloud in two point clouds, given the equation of a plane.
// one point cloud will have all the points above the plane and the other with all the points below the plane.
// Points exactly on the plane are not included!
func SplitPointCloudByPlane(cloud pc.PointCloud, plane pc.Plane) (pc.PointCloud, pc.PointCloud, error) {
	aboveCloud, belowCloud := pc.New(), pc.New()
	var err error
	cloud.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		dist := plane.Distance(pt)
		if plane.Equation()[2] > 0.0 {
			dist = -dist
		}
		if dist > 0.0 {
			err = aboveCloud.Set(pt, d)
		} else if dist < 0.0 {
			err = belowCloud.Set(pt, d)
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
	cloud.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		dist := plane.Distance(pt)
		if math.Abs(dist) <= threshold {
			err = thresholdCloud.Set(pt, d)
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
	visitedPoints := make(map[r3.Vector]bool)
	var err error
	for i, cloud := range segments {
		cloud.Iterate(0, 0, func(pos r3.Vector, d pc.Data) bool {
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
