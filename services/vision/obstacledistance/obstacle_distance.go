package obstacledistance

import (
	"context"

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

var model = resource.DefaultModelFamily.WithModel("obstacle_distance_detector")

type ObstacleDistanceDetectorConfig struct {
	resource.TriviallyValidateConfig
	DetectorName string `json:"detector_name"`
	// ConfidenceThresh float64 `json:"confidence_threshold_pct"`
	NumQueries int `json:"num_queries"`
	// add field for number of queries
}

func init() {
	resource.RegisterService(svision.API, model, resource.Registration[svision.Service, *ObstacleDistanceDetectorConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (svision.Service, error) {
			attrs, err := resource.NativeConfig[*ObstacleDistanceDetectorConfig](c)
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

func registerObstacleDistanceDetector(
	ctx context.Context,
	name resource.Name,
	conf *ObstacleDistanceDetectorConfig,
	r robot.Robot,
) (svision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDistanceDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("object detection config for distance detector cannot be nil")
	}
	usSensor, err := camera.FromRobot(r, conf.DetectorName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find necessary dependency, detector %q", conf.DetectorName)
	}
	if conf.NumQueries < 1 || conf.NumQueries > 20 {
		return nil, errors.New("invalid number of queries, pick a number between 1 and 20")
	}

	segmenter := func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		clouds := []pointcloud.PointCloud{}

		for i := 0; i < conf.NumQueries; i++ {
			nxtPC, err := usSensor.NextPointCloud(ctx)
			if err != nil {
				return nil, err
			}
			clouds[i] = nxtPC
		}

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
			return nil, err
		}

		values := []float64{}
		count := 0

		mergedCloud.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			values = append(values, p.Z)
			count++
			return true
		})
		if count > conf.NumQueries {
			return nil, errors.New("more than one point from one of the readings, expected one")
		}

		median, err := stats.Median(values)
		if err != nil {
			return nil, err
		}

		vector := pointcloud.NewVector(0, 0, median)

		pt := spatialmath.NewPoint(vector, "obstacle").Pose()

		sphere, err := spatialmath.NewSphere(pt, 0, "obstacle")
		if err != nil {
			return nil, err
		}

		pcToReturn := pointcloud.New()
		basicData := pointcloud.NewBasicData()
		err = pcToReturn.Set(vector, basicData)
		if err != nil {
			return nil, err
		}

		// when iterating if more than one point, return error
		// query nextpointcloud multiple times, take median?
		// implementation of kalman filter (smart smoothing average function) over readings from nextpointcloud
		// return pointcloud and geometry

		toReturn := make([]*vision.Object, 0)
		toReturn[0] = &vision.Object{PointCloud: pcToReturn, Geometry: sphere}

		return toReturn, nil
	}
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}
