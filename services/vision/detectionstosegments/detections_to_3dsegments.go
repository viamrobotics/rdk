package detectionstosegments

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.NewDefaultModel("detections_to_3dsegments")

func init() {
	registry.RegisterService(vision.Subtype, model, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			attrs, ok := c.ConvertedAttributes.(*segmentation.DetectionSegmenterConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, c.ConvertedAttributes)
			}
			return register3DSegmenterFromDetector(ctx, c.Name, attrs, r, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(
		vision.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf segmentation.DetectionSegmenterConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*segmentation.DetectionSegmenterConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&segmentation.DetectionSegmenterConfig{},
	)
}

type detector2segmenter struct {
	objectdetection.Detector
	segmentation.Segmenter
}

func (ds *detector2segmenter) Segment(ctx context.Context, c camera.Camera) ([]*viz.Object, error) {
	return ds.Segmenter(ctx, c)
}

func (ds *detector2segmenter) Detect(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
	return ds.Detector(ctx, img)
}

// register3DSegmenterFromDetector creates a 3D segmenter from a previously registered detector
func register3DSegmenterFromDetector(ctx context.Context, name string, conf *segmentation.DetectionSegmenterConfig, r robot.Robot, logger golog.Logger) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::register3DSegmenterFromDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for 3D segmenter made from a detector cannot be nil")
	}
	detectorService, err := vision.FromRobot(r, conf.DetectorName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find necessary dependency, detector %q", conf.DetectorName)
	}
	confThresh := 0.5 // default value
	if conf.ConfidenceThresh > 0.0 {
		confThresh = conf.ConfidenceThresh
	}
	detector := func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		return detectorService.Detections(ctx, img, nil)
	}
	segmenter, err := segmentation.DetectionSegmenter(objectdetection.Detector(detector), conf.MeanK, conf.Sigma, confThresh)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create 3D segmenter from detector")
	}
	return vision.NewService(name, &detector2segmenter{detector, segmenter}, r)
}
