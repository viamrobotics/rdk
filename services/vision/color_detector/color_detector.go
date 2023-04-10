package colordetector

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

var model = resource.NewDefaultModel("color_detector")

func init() {
	registry.RegisterService(vision.Subtype, model, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			attrs, ok := c.ConvertedAttributes.(*objdet.ColorDetectorConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, c.ConvertedAttributes)
			}
			return registerColorDetector(ctx, c.Name, attrs, r, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(
		vision.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf objdet.ColorDetectorConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*objdet.ColorDetectorConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&objdet.ColorDetectorConfig{},
	)
}

type colorDetector struct {
	objectdetection.Detector
}

func (cd *colorDetector) Detect(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
	return cd.Detector(ctx, img)
}

// registerColorDetector creates a new Color Detector from the config
func registerColorDetector(ctx context.Context, name string, conf *objdet.ColorDetectorConfig, r robot.Robot, logger golog.Logger) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerColorDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("object detection config for color detector cannot be nil")
	}
	detector, err := objdet.NewColorDetector(conf)
	if err != nil {
		return nil, errors.Wrapf(err, "register color detector %s", name)
	}
	return vision.NewService(name, &colorDetector{detector}, r)
}
