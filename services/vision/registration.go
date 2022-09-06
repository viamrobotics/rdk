package vision

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

// registerColorDetector parses the Parameter field from the config into ColorDetectorConfig,
// creates the ColorDetector, and registers it to the detector map.
func registerColorDetector(ctx context.Context, mm modelMap, conf *VisModelConfig, logger golog.Logger) error {
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
	regModel := registeredModel{model: detector, modelType: ColorDetector, closer: nil}
	return mm.registerVisModel(conf.Name, &regModel, logger)
}

func registerTfliteClassifier(ctx context.Context, mm modelMap, conf *VisModelConfig, logger golog.Logger) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::registerTfliteClassifier")
	defer span.End()
	if conf == nil {
		return errors.New("object detection config for tflite classifier cannot be nil")
	}
	classifier, model, err := NewTFLiteClassifier(ctx, conf, logger)
	if err != nil {
		return errors.Wrapf(err, "could not register tflite classifier %s", conf.Name)
	}

	regModel := registeredModel{model: classifier, modelType: TFLiteClassifier, closer: model}
	return mm.registerVisModel(conf.Name, &regModel, logger)
}

func registerTfliteDetector(ctx context.Context, mm modelMap, conf *VisModelConfig, logger golog.Logger) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::registerTfliteDetector")
	defer span.End()
	if conf == nil {
		return errors.New("object detection config for tflite detector cannot be nil")
	}
	detector, model, err := NewTFLiteDetector(ctx, conf, logger)
	if err != nil {
		return errors.Wrapf(err, "could not register tflite detector %s", conf.Name)
	}

	regModel := registeredModel{model: detector, modelType: TFLiteDetector, closer: model}
	return mm.registerVisModel(conf.Name, &regModel, logger)
}

func registerRCSegmenter(ctx context.Context, mm modelMap, conf *VisModelConfig, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerRCSegmenter")
	defer span.End()
	if conf == nil {
		return errors.New("config for radius clustering segmenter cannot be nil")
	}
	segmenter := segmentation.Segmenter(segmentation.RadiusClustering)

	regModel := registeredModel{model: segmenter, modelType: RCSegmenter, closer: nil}
	return mm.registerVisModel(conf.Name, &regModel, logger)
}
