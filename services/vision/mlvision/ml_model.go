// Package mlvision uses an underlying model from the ML model service as a vision model,
// and wraps the ML model with the vision service methods.
package mlvision

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("ml_model")

func init() {
	registry.RegisterService(vision.Subtype, model, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			attrs, ok := c.ConvertedAttributes.(*MLModelConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, c.ConvertedAttributes)
			}
			return registerMLModelVisionService(ctx, c.Name, attrs, r, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(
		vision.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf MLModelConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*MLModelConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&MLModelConfig{},
	)
}

// MLModelConfig specifies the parameters needed to turn an ML model into a vision Model.
type MLModelConfig struct {
	ModelName string `json:"ml_model_name"`
}

func registerMLModelVisionService(
	ctx context.Context,
	name string,
	params *MLModelConfig,
	r robot.Robot,
	logger golog.Logger,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerMLModelVisionService")
	defer span.End()

	mlm, err := mlmodel.FromRobot(r, params.ModelName)
	if err != nil {
		return nil, err
	}
	classifierFunc, err := attemptToBuildClassifier(mlm)
	if err != nil {
		logger.Infof("was not able to turn ml model %q into a classifier", params.ModelName)
	}
	detectorFunc, err := attemptToBuildDetector(mlm)
	if err != nil {
		logger.Infof("was not able to turn ml model %q into a detector", params.ModelName)
	}
	segmenter3DFunc, err := attemptToBuild3DSegmenter(mlm)
	if err != nil {
		logger.Infof("was not able to turn ml model %q into a 3D segmenter", params.ModelName)
	}
	// Don't return a close function, because you don't want to close the underlying ML service
	return vision.NewService(name, r, nil, classifierFunc, detectorFunc, segmenter3DFunc)
}
