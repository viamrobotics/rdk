package obstacledistance

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	svision "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"

	vision "go.viam.com/rdk/vision"
)

var model = resource.DefaultModelFamily.WithModel("obstacle_distance_detector")

type ObstacleDistanceDetectorConfig struct {
	resource.TriviallyValidateConfig
	DetectorName     string  `json:"detector_name"`
	ConfidenceThresh float64 `json:"confidence_threshold_pct"`
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
	segmenter := func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		nxtPC, err := usSensor.NextPointCloud(ctx)
		if err != nil {
			return nil, err
		}
		obj, err := vision.NewObject(nxtPC)
		if err != nil {
			return nil, err
		}
		// look at scopedoc n  fix

		return []*vision.Object{obj}, nil
	}
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}
