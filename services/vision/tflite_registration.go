package vision

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

func registerTfliteDetector(ctx context.Context, dm detectorMap, conf *DetectorConfig, logger golog.Logger) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::registerTfliteDetector")
	defer span.End()
	if conf == nil {
		return errors.New("object detection config for tflite detector cannot be nil")
	}
	detector, model, err := NewTFLiteDetector(ctx, conf, logger)
	if err != nil {
		return errors.Wrapf(err, "could not register tflite detector %s", conf.Name)
	}

	regDetector := registeredDetector{detector: detector, closer: model}

	return dm.registerDetector(conf.Name, &regDetector, logger)
}
