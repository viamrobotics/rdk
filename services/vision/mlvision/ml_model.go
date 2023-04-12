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
	ctx, span := trace.StartSpan(ctx, "service::vision::registerMLModelVisionService")
	defer span.End()

	mlm, err := mlmodel.FromRobot(r, params.ModelName)
	if err != nil {
		return nil, err
	}
	classifierFunc, err := attemptToBuildClassifier(mlm)
	if err != nil {
		return nil, err
	}
	detectorFunc, err := attemptToBuildDetector(mlm)
	if err != nil {
		return nil, err
	}
	segmenter3DFunc, err := attemptToBuild3DSegmenter(mlm)
	if err != nil {
		return nil, err
	}
	// Don't return the model, because you don't want to close the underlying service
	return vision.NewService(name, r, nil, classifierFunc, detectorFunc, segmenter3DFunc)
}
