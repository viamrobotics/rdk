package vision

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

// VisModelType defines what vision models are known by the vision service.
type VisModelType string

// VisOperation defines what types of operations are allowed by the vision service.
type VisOperation string

// The set of allowed vision model types.
const (
	TFLiteDetector    = VisModelType("tflite_detector")
	TFDetector        = VisModelType("tf_detector")
	ColorDetector     = VisModelType("color_detector")
	TFLiteClassifier  = VisModelType("tflite_classifier")
	TFClassifier      = VisModelType("tf_classifier")
	RCSegmenter       = VisModelType("radius_clustering_segmenter")
	DetectorSegmenter = VisModelType("detector_segmenter")
)

// RegisteredModelParameterSchemas maps the vision model types to the necessary parameters needed to create them.
var RegisteredModelParameterSchemas = map[VisModelType]*jsonschema.Schema{
	TFLiteDetector:    jsonschema.Reflect(&TFLiteDetectorConfig{}),
	ColorDetector:     jsonschema.Reflect(&objectdetection.ColorDetectorConfig{}),
	TFLiteClassifier:  jsonschema.Reflect(&TFLiteClassifierConfig{}),
	RCSegmenter:       jsonschema.Reflect(&segmentation.RadiusClusteringConfig{}),
	DetectorSegmenter: jsonschema.Reflect(&segmentation.DetectionSegmenterConfig{}),
}

// The set of operations supported by the vision model types.
const (
	VisDetection      = VisOperation("detection")
	VisClassification = VisOperation("classification")
	VisSegmentation   = VisOperation("segmentation")
)

// visModelToOpMap maps the vision model type with the corresponding vision operation.
var visModelToOpMap = map[VisModelType]VisOperation{
	TFLiteDetector:    VisDetection,
	TFDetector:        VisDetection,
	ColorDetector:     VisDetection,
	TFLiteClassifier:  VisClassification,
	TFClassifier:      VisClassification,
	RCSegmenter:       VisSegmentation,
	DetectorSegmenter: VisSegmentation,
}

// newVisModelTypeNotImplemented is used when the model type is not implemented.
func newVisModelTypeNotImplemented(name string) error {
	return errors.Errorf("vision model type %q is not implemented", name)
}

// VisModelConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type VisModelConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

type modelMap map[string]registeredModel

type registeredModel struct {
	model     interface{}
	modelType VisModelType
	closer    io.Closer
}

func (m *registeredModel) toDetector() (objectdetection.Detector, error) {
	toReturn, ok := m.model.(objectdetection.Detector)
	if !ok {
		return nil, errors.New("couldn't convert model to detector")
	}
	return toReturn, nil
}

func (m *registeredModel) toClassifier() (classification.Classifier, error) {
	toReturn, ok := m.model.(classification.Classifier)
	if !ok {
		return nil, errors.New("couldn't convert model to classifier")
	}
	return toReturn, nil
}

func (m *registeredModel) toSegmenter() (segmentation.Segmenter, error) {
	toReturn, ok := m.model.(segmentation.Segmenter)
	if !ok {
		return nil, errors.New("couldn't convert model to segmenter")
	}
	return toReturn, nil
}

func (mm modelMap) DetectorNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		thisType, err := mm.getModelType(name)
		if err == nil { // found the model
			if visModelToOpMap[thisType] == VisDetection {
				names = append(names, name)
			}
		}
	}
	return names
}

func (mm modelMap) ClassifierNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		thisType, err := mm.getModelType(name)
		if err == nil {
			if visModelToOpMap[thisType] == VisClassification {
				names = append(names, name)
			}
		}
	}
	return names
}

func (mm modelMap) SegmenterNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		thisType, err := mm.getModelType(name)
		if err == nil {
			if visModelToOpMap[thisType] == VisSegmentation {
				names = append(names, name)
			}
		}
	}
	return names
}

func (mm modelMap) getModelType(name string) (VisModelType, error) {
	m, ok := mm[name]
	if !ok {
		return "", errors.Errorf("no such vision model with name %q", name)
	}
	return m.modelType, nil
}

func (mm modelMap) modelLookup(name string) (registeredModel, error) {
	m, ok := mm[name]
	if !ok {
		return registeredModel{}, errors.Errorf("no such vision model with name %q", name)
	}
	return m, nil
}

func (mm modelMap) modelNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		names = append(names, name)
	}
	return names
}

func (mm modelMap) removeVisModel(name string, logger golog.Logger) error {
	if _, ok := mm[name]; !ok {
		logger.Infof("no such vision model with name %s", name)
		return nil
	}

	if mm[name].closer != nil {
		err := mm[name].closer.Close()
		if err != nil {
			return err
		}
	}
	delete(mm, name)
	return nil
}

func (mm modelMap) registerVisModel(name string, m *registeredModel, logger golog.Logger) error {
	if m == nil || m.model == nil {
		return errors.Errorf("cannot register a nil model: %s", name)
	}
	if m.closer != nil {
		mm[name] = registeredModel{
			model: m.model, modelType: m.modelType, closer: m.closer,
		}
		return nil
	}
	if _, old := mm[name]; old {
		logger.Infof("overwriting the model with name: %s", name)
	}

	mm[name] = registeredModel{
		model: m.model, modelType: m.modelType, closer: nil,
	}
	return nil
}

// registerNewDetectors take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func registerNewVisModels(ctx context.Context, mm modelMap, attrs *Attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerNewVisModels")
	defer span.End()
	for _, attr := range attrs.ModelRegistry {
		logger.Debugf("adding vision model %q of type %q", attr.Name, attr.Type)
		switch VisModelType(attr.Type) {
		case TFLiteDetector:
			return registerTfliteDetector(ctx, mm, &attr, logger)
		case TFLiteClassifier:
			return registerTfliteClassifier(ctx, mm, &attr, logger)
		case TFDetector:
			return newVisModelTypeNotImplemented(attr.Type)
		case TFClassifier:
			return newVisModelTypeNotImplemented(attr.Type)
		case ColorDetector:
			return registerColorDetector(ctx, mm, &attr, logger)
		case RCSegmenter:
			return registerRCSegmenter(ctx, mm, &attr, logger)
		case DetectorSegmenter:
			return registerSegmenterFromDetector(ctx, mm, &attr, logger)
		default:
			return newVisModelTypeNotImplemented(attr.Type)
		}
	}
	return nil
}
