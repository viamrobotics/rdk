// Package obstaclesdistance uses an underlying camera to fulfill vision service methods, specifically
// GetObjectPointClouds, which performs several queries of NextPointCloud and returns a median point.
package obstaclesdistance

import (
	"context"
	"math"
	"sort"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	svision "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	vision "go.viam.com/rdk/vision"
)

var model = resource.DefaultModelFamily.WithModel("obstacle_distance")

// DefaultNumQueries is the default number of times the camera should be queried before averaging
const DefaultNumQueries = 10

// DistanceDetectorConfig specifies the parameters for the camera to be used
// for the obstacle distance detection service.
type DistanceDetectorConfig struct {
	NumQueries int `json:"num_queries"`
}

func init() {
	resource.RegisterService(svision.API, model, resource.Registration[svision.Service, *DistanceDetectorConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (svision.Service, error) {
			attrs, err := resource.NativeConfig[*DistanceDetectorConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerObstacleDistanceDetector(ctx, c.ResourceName(), attrs, actualR)
		},
	})
}

// Validate ensures all parts of the config are valid.
func (config *DistanceDetectorConfig) Validate(path string) ([]string, error) {
	deps := []string{}
	if config.NumQueries == 0 {
		config.NumQueries = DefaultNumQueries
	}
	if config.NumQueries < 1 || config.NumQueries > 20 {
		return nil, errors.New("invalid number of queries, pick a number between 1 and 20")
	}
	return deps, nil
}

func registerObstacleDistanceDetector(
	ctx context.Context,
	name resource.Name,
	conf *DistanceDetectorConfig,
	r robot.Robot,
) (svision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDistanceDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacle_distance cannot be nil")
	}

	segmenter := func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		clouds := make([]pointcloud.PointCloud, 0, conf.NumQueries)

		for i := 0; i < conf.NumQueries; i++ {
			nxtPC, err := src.NextPointCloud(ctx)
			if err != nil {
				return nil, err
			}
			if nxtPC.Size() == 0 {
				continue
			}
			clouds = append(clouds, nxtPC)
		}
		if len(clouds) == 0 {
			return nil, errors.New("none of the input point clouds contained any points")
		}

		median, err := medianFromPointClouds(ctx, clouds)
		if err != nil {
			return nil, err
		}

		// package the result into a vision.Object
		vector := pointcloud.NewVector(median.X, median.Y, median.Z)
		pt := spatialmath.NewPoint(vector, "obstacle")

		pcToReturn := pointcloud.New()
		basicData := pointcloud.NewBasicData()
		err = pcToReturn.Set(vector, basicData)
		if err != nil {
			return nil, err
		}

		toReturn := make([]*vision.Object, 1)
		toReturn[0] = &vision.Object{PointCloud: pcToReturn, Geometry: pt}

		return toReturn, nil
	}
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

func medianFromPointClouds(ctx context.Context, clouds []pointcloud.PointCloud) (r3.Vector, error) {
	var results [][]r3.Vector // a slice for each process, which will contain a slice of vectors
	err := utils.GroupWorkParallel(
		ctx,
		len(clouds),
		func(numGroups int) {
			results = make([][]r3.Vector, numGroups)
		},
		func(groupNum, groupSize, from, to int) (utils.MemberWorkFunc, utils.GroupWorkDoneFunc) {
			closestPoints := make([]r3.Vector, 0, groupSize)
			return func(memberNum, workNum int) {
					closestPoint := getClosestPoint(clouds[workNum])
					closestPoints = append(closestPoints, closestPoint)
				}, func() {
					results[groupNum] = closestPoints
				}
		},
	)
	if err != nil {
		return r3.Vector{}, err
	}
	candidates := make([]r3.Vector, 0, len(clouds))
	for _, r := range results {
		candidates = append(candidates, r...)
	}
	if len(candidates) == 0 {
		return r3.Vector{}, errors.New("point cloud list is empty, could not find median point")
	}
	return getMedianPoint(candidates), nil
}

func getClosestPoint(cloud pointcloud.PointCloud) r3.Vector {
	minDistance := math.MaxFloat64
	minPoint := r3.Vector{}
	cloud.Iterate(0, 0, func(pt r3.Vector, d pointcloud.Data) bool {
		dist := pt.Norm2()
		if dist < minDistance {
			minDistance = dist
			minPoint = pt
		}
		return true
	})
	return minPoint
}

// to calculate the median, will need to sort the vectors by distance from origin.
func sortVectors(v []r3.Vector) {
	sort.Sort(points(v))
}

type points []r3.Vector

func (p points) Len() int           { return len(p) }
func (p points) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p points) Less(i, j int) bool { return p[i].Norm2() < p[j].Norm2() }

func getMedianPoint(pts []r3.Vector) r3.Vector {
	sortVectors(pts)
	index := (len(pts) - 1) / 2
	return pts[index]
}
