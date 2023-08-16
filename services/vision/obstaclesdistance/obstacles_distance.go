// Package obstaclesdistance uses an underlying camera to fulfill vision service methods, specifically
// GetObjectPointClouds, which performs several queries of NextPointCloud and returns a median point.
package obstaclesdistance

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/montanaflynn/stats"
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
		clouds := make([]pointcloud.PointCloud, conf.NumQueries)

		for i := 0; i < conf.NumQueries; i++ {
			nxtPC, err := src.NextPointCloud(ctx)
			if err != nil {
				return nil, err
			}
			clouds[i] = nxtPC
			if nxtPC.Size() != 1 {
				return nil, errors.New("obstacles_distance expects one point in the point cloud from the camera." +
					fmt.Sprintf(" Underlying camera generates %d point(s) in its point cloud", nxtPC.Size()))
			}
		}

		median, err := medianFromPointClouds(clouds)
		if err != nil {
			return nil, err
		}

		vector := pointcloud.NewVector(0, 0, median)

		pt := spatialmath.NewPoint(vector, "obstacle")

		pcToReturn := pointcloud.New()
		basicData := pointcloud.NewBasicData()
		err = pcToReturn.Set(vector, basicData)
		if err != nil {
			return nil, err
		}

		// implementation of kalman filter (smart smoothing average function) over readings from nextpointcloud

		toReturn := make([]*vision.Object, 1)
		toReturn[0] = &vision.Object{PointCloud: pcToReturn, Geometry: pt}

		return toReturn, nil
	}
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

func medianFromPointClouds(clouds []pointcloud.PointCloud) (float64, error) {
	cloudsWithOffset := make([]pointcloud.CloudAndOffsetFunc, 0, len(clouds))
	for _, cloud := range clouds {
		cloudCopy := cloud
		cloudFunc := func(ctx context.Context) (pointcloud.PointCloud, spatialmath.Pose, error) {
			return cloudCopy, nil, nil
		}
		cloudsWithOffset = append(cloudsWithOffset, cloudFunc)
	}

	mergedCloud, err := pointcloud.MergePointClouds(context.Background(), cloudsWithOffset, nil)
	if err != nil {
		return -1, err
	}

	values := []float64{}

	mergedCloud.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		values = append(values, p.Z)
		return true
	})

	median, err := stats.Median(values)
	if err != nil {
		return -1, err
	}

	return median, err
}
