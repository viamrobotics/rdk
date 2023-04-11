package mlmodel

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
	goutils "go.viam.com/utils"
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

type mlModelVisionService struct {
	mlm          mlmodel.Service
	r            robot.Robot
	classifyFunc classification.Classifier
	detectFunc   objectdetection.Detector
	segmentFunc  segmentation.Segmenter
	generic.Unimplemented
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
	_, err = attemptToBuildClassifier(mlm)
	if err != nil {
		return nil, err
	}
	return &mlModelVisionService{
		mlm:          mlm,
		r:            r,
		classifyFunc: nil,
		detectFunc:   nil,
		segmentFunc:  nil,
	}, nil
}

func attemptToBuildClassifier(mlm mlmodel.Service) (classification.Classifier, error) {
	return nil, nil
}

func attemptToBuildDetector(mlm mlmodel.Service) (objectdetection.Detector, error) {
	return nil, nil
}

func attemptToBuild3DSegmenter(mlm mlmodel.Service) (segmentation.Segmenter, error) {
	return nil, nil
}

// Detections returns the detections of given image if the model implements objectdetector.Detector.
func (ml *mlModelVisionService) Detections(
	ctx context.Context,
	img image.Image,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	return nil, nil
}

// DetectionsFromCamera returns the detections of the next image from the given camera.
func (ml *mlModelVisionService) DetectionsFromCamera(
	ctx context.Context,
	cameraName string,
	extra map[string]interface{},
) ([]objectdetection.Detection, error) {
	return nil, nil
}

// Classifications returns the classifications of given image if the model implements classifications.Classifier
func (ml *mlModelVisionService) Classifications(
	ctx context.Context,
	img image.Image,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	return nil, nil
}

// ClassificationsFromCamera returns the classifications of the next image from the given camera.
func (ml *mlModelVisionService) ClassificationsFromCamera(
	ctx context.Context,
	cameraName string,
	n int,
	extra map[string]interface{},
) (classification.Classifications, error) {
	return nil, nil
}

// GetObjectPointClouds returns all the found objects in a 3D image if the model implements Segmenter3D
func (ml *mlModelVisionService) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
	return nil, nil
}

func (ml *mlModelVisionService) Close(ctx context.Context) error {
	return goutils.TryClose(ctx, ml.mlm)
}
