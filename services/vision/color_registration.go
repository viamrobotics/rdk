package vision

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// registerColorDetector parses the Parameter field from the config into ColorDetectorConfig,
// creates the ColorDetector, and registers it to the detector map.
func registerColorDetector(ctx context.Context, dm detectorMap, conf *DetectorConfig, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerColorDetector")
	defer span.End()
	if conf == nil {
		return errors.New("object detection config for color detector cannot be nil")
	}
	var p objdet.ColorDetectorConfig
	attrs, err := config.TransformAttributeMapToStruct(&p, conf.Parameters)
	if err != nil {
		return errors.Wrapf(err, "register color detector %s", conf.Name)
	}
	params, ok := attrs.(*objdet.ColorDetectorConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, attrs)
		return errors.Wrapf(err, "register color detector %s", conf.Name)
	}
	detector, err := objdet.NewColorDetector(params)
	if err != nil {
		return errors.Wrapf(err, "register color detector %s", conf.Name)
	}
	regDetector := registeredDetector{detector: detector, closer: nil}
	return dm.registerDetector(conf.Name, &regDetector, logger)
}
