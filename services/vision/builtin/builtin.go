//go:build !arm && !windows

// Package builtin is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
package builtin

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterService(vision.Subtype, resource.DefaultServiceModel, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	config.RegisterServiceAttributeMapConverter(vision.Subtype, resource.DefaultServiceModel,
		func(attributeMap config.AttributeMap) (interface{}, error) {
			var attrs vision.Attributes
			return config.TransformAttributeMapToStruct(&attrs, attributeMap)
		},
		&vision.Attributes{},
	)
	resource.AddDefaultService(vision.Named(resource.DefaultServiceName))
}

// RadiusClusteringSegmenter is  the name of a segmenter that finds well separated objects on a flat plane.
const RadiusClusteringSegmenter = "radius_clustering"

// NewBuiltIn registers new detectors from the config and returns a new object detection service for the given robot.
func NewBuiltIn(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (vision.Service, error) {
	modMap := make(modelMap)
	// register detectors and user defined things if config is defined
	if config.ConvertedAttributes != nil {
		attrs, ok := config.ConvertedAttributes.(*vision.Attributes)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
		}
		err := registerNewVisModels(ctx, modMap, attrs, logger)
		if err != nil {
			return nil, err
		}
	}
	service := &builtIn{
		r:      r,
		modReg: modMap,
		logger: logger,
	}
	return service, nil
}

type builtIn struct {
	generic.Unimplemented
	r      robot.Robot
	modReg modelMap
	logger golog.Logger
}

// GetModelParameterSchema takes the model name and returns the parameters needed to add one to the vision registry.
func (vs *builtIn) GetModelParameterSchema(
	ctx context.Context,
	modelType vision.VisModelType,
	extra map[string]interface{},
) (*jsonschema.Schema, error) {
	if modelSchema, ok := registeredModelParameterSchemas[modelType]; ok {
		if modelSchema == nil {
			return nil, errors.Errorf("do not have a schema for model type %q", modelType)
		}
		return modelSchema, nil
	}
	return nil, errors.Errorf("do not have a schema for model type %q", modelType)
}

// Detection Methods
// DetectorNames returns a list of the all the names of the detectors in the registry.
func (vs *builtIn) DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::DetectorNames")
	defer span.End()
	return vs.modReg.DetectorNames(), nil
}

// AddDetector adds a new detector from an Attribute config struct.
func (vs *builtIn) AddDetector(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::AddDetector")
	defer span.End()
	attrs := &vision.Attributes{ModelRegistry: []vision.VisModelConfig{cfg}}
	err := registerNewVisModels(ctx, vs.modReg, attrs, vs.logger)
	if err != nil {
		return err
	}
	// automatically add a segmenter version of detector
	segmenterConf := vision.VisModelConfig{
		Name:       cfg.Name + "_segmenter",
		Type:       string(DetectorSegmenter),
		Parameters: config.AttributeMap{"detector_name": cfg.Name, "mean_k": 0, "sigma": 0.0},
	}
	return vs.AddSegmenter(ctx, segmenterConf, extra)
}

// RemoveDetector removes a detector from the registry.
func (vs *builtIn) RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error {
	_, span := trace.StartSpan(ctx, "service::vision::RemoveDetector")
	defer span.End()
	err := vs.modReg.removeVisModel(detectorName, vs.logger)
	if err != nil {
		return err
	}
	// remove the associated segmenter as well (if there is one)
	return vs.RemoveSegmenter(ctx, detectorName+"_segmenter", extra)
}

// DetectionsFromCamera returns the detections of the next image from the given camera and the given detector.
func (vs *builtIn) DetectionsFromCamera(
	ctx context.Context,
	cameraName, detectorName string,
	extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::DetectionsFromCamera")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	d, err := vs.modReg.modelLookup(detectorName)
	if err != nil {
		return nil, err
	}
	detector, err := d.toDetector()
	if err != nil {
		return nil, err
	}
	img, release, err := camera.ReadImage(ctx, cam)
	if err != nil {
		return nil, err
	}
	defer release()

	return detector(ctx, img)
}

// Detections returns the detections of given image using the given detector.
func (vs *builtIn) Detections(ctx context.Context, img image.Image, detectorName string, extra map[string]interface{},
) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Detections")
	defer span.End()

	d, err := vs.modReg.modelLookup(detectorName)
	if err != nil {
		return nil, err
	}
	detector, err := d.toDetector()
	if err != nil {
		return nil, err
	}

	return detector(ctx, img)
}

// ClassifierNames returns a list of the all the names of the classifiers in the registry.
func (vs *builtIn) ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::ClassifierNames")
	defer span.End()
	return vs.modReg.ClassifierNames(), nil
}

// AddClassifier adds a new classifier from an Attribute config struct.
func (vs *builtIn) AddClassifier(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::AddClassifier")
	defer span.End()
	attrs := &vision.Attributes{ModelRegistry: []vision.VisModelConfig{cfg}}
	err := registerNewVisModels(ctx, vs.modReg, attrs, vs.logger)
	if err != nil {
		return err
	}
	return nil
}

// Remove classifier removes a classifier from the registry.
func (vs *builtIn) RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error {
	_, span := trace.StartSpan(ctx, "service::vision::RemoveClassifier")
	defer span.End()
	err := vs.modReg.removeVisModel(classifierName, vs.logger)
	if err != nil {
		return err
	}
	return nil
}

// ClassificationsFromCamera returns the classifications of the next image from the given camera and the given detector.
func (vs *builtIn) ClassificationsFromCamera(ctx context.Context, cameraName,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::ClassificationsFromCamera")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	c, err := vs.modReg.modelLookup(classifierName)
	if err != nil {
		return nil, err
	}
	classifier, err := c.toClassifier()
	if err != nil {
		return nil, err
	}
	img, release, err := camera.ReadImage(ctx, cam)
	if err != nil {
		return nil, err
	}
	defer release()
	fullClassifications, err := classifier(ctx, img)
	if err != nil {
		return nil, err
	}
	return fullClassifications.TopN(n)
}

// Classifications returns the classifications of given image using the given classifier.
func (vs *builtIn) Classifications(ctx context.Context, img image.Image,
	classifierName string, n int, extra map[string]interface{},
) (classification.Classifications, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::Classifications")
	defer span.End()

	c, err := vs.modReg.modelLookup(classifierName)
	if err != nil {
		return nil, err
	}
	classifier, err := c.toClassifier()
	if err != nil {
		return nil, err
	}
	fullClassifications, err := classifier(ctx, img)
	if err != nil {
		return nil, err
	}
	return fullClassifications.TopN(n)
}

// Segmentation Methods
// SegmenterNames returns a list of all the names of the segmenters in the segmenter map.
func (vs *builtIn) SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::SegmenterNames")
	defer span.End()
	return vs.modReg.SegmenterNames(), nil
}

// AddSegmenter adds a new segmenter from an Attribute config struct.
func (vs *builtIn) AddSegmenter(ctx context.Context, cfg vision.VisModelConfig, extra map[string]interface{}) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::AddSegmenter")
	defer span.End()
	attrs := &vision.Attributes{ModelRegistry: []vision.VisModelConfig{cfg}}
	return registerNewVisModels(ctx, vs.modReg, attrs, vs.logger)
}

// RemoveSegmenter removes a segmenter from the registry.
func (vs *builtIn) RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error {
	_, span := trace.StartSpan(ctx, "service::vision::RemoveSegmenter")
	defer span.End()
	return vs.modReg.removeVisModel(segmenterName, vs.logger)
}

// GetObjectPointClouds returns all the found objects in a 3D image according to the chosen segmenter.
func (vs *builtIn) GetObjectPointClouds(
	ctx context.Context,
	cameraName string,
	segmenterName string, extra map[string]interface{},
) ([]*viz.Object, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetObjectPointClouds")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	s, err := vs.modReg.modelLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	segmenter, err := s.toSegmenter()
	if err != nil {
		return nil, err
	}
	return segmenter(ctx, cam)
}

// Close removes all existing detectors from the vision service.
func (vs *builtIn) Close() error {
	models := vs.modReg.ModelNames()
	for _, detectorName := range models {
		err := vs.modReg.removeVisModel(detectorName, vs.logger)
		if err != nil {
			return err
		}
	}
	return nil
}
