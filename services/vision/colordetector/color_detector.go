// Package colordetector uses a heuristic based on hue and connected components to create
// bounding boxes around objects of a specified color.
package colordetector

import (
	"context"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

var model = resource.DefaultModelFamily.WithModel("color_detector")

func init() {
	resource.RegisterService(vision.API, model, resource.Registration[vision.Service, *objdet.ColorDetectorConfig]{
		DeprecatedRobotConstructor: func(
			ctx context.Context, r any, c resource.Config, logger logging.Logger,
		) (vision.Service, error) {
			attrs, err := resource.NativeConfig[*objdet.ColorDetectorConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerColorDetector(ctx, c.ResourceName(), attrs, actualR)
		},
	})
}

// registerColorDetector creates a new Color Detector from the config.
func registerColorDetector(
	ctx context.Context,
	name resource.Name,
	conf *objdet.ColorDetectorConfig,
	r robot.Robot,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerColorDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("object detection config for color detector cannot be nil")
	}
	detector, err := objdet.NewColorDetector(conf)
	if err != nil {
		return nil, errors.Wrapf(err, "error registering color detector %q", name)
	}
	return vision.NewService(name, r, nil, nil, detector, nil)
}
