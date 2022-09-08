package vision

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
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
	TFLiteDetector   = VisModelType("tflite_detector")
	TFDetector       = VisModelType("tf_detector")
	ColorDetector    = VisModelType("color_detector")
	TFLiteClassifier = VisModelType("tflite_classifier")
	TFClassifier     = VisModelType("tf_classifier")
	RCSegmenter      = VisModelType("radius_clustering_segmenter")
	ObjectSegmenter  = VisModelType("object_segmenter")
)

// The set of operations supported by the vision model types.
const (
	VisDetection      = VisOperation("detection")
	VisClassification = VisOperation("classification")
	VisSegmentation   = VisOperation("segmentation")
)

// visModelToOpMap maps the vision model type with the corresponding vision operation.
var visModelToOpMap = map[VisModelType]VisOperation{
	TFLiteDetector:   VisDetection,
	TFDetector:       VisDetection,
	ColorDetector:    VisDetection,
	TFLiteClassifier: VisClassification,
	TFClassifier:     VisClassification,
	RCSegmenter:      VisSegmentation,
	ObjectSegmenter:  VisSegmentation,
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

// ModelMap is a map that holds registered models.
type ModelMap map[string]RegisteredModel

// RegisteredModel struct that holds models parameters.
type RegisteredModel struct {
	Model     interface{}
	ModelType VisModelType
	Closer    io.Closer
	SegParams []utils.TypedName
}

// ToDetector converts model to a dectector.
func (m *RegisteredModel) ToDetector() (objectdetection.Detector, error) {
	toReturn, ok := m.Model.(objectdetection.Detector)
	if !ok {
		return nil, errors.New("couldn't convert model to detector")
	}
	return toReturn, nil
}

// ToClassifier converts model to a classifier.
func (m *RegisteredModel) ToClassifier() (classification.Classifier, error) {
	toReturn, ok := m.Model.(classification.Classifier)
	if !ok {
		return nil, errors.New("couldn't convert model to classifier")
	}
	return toReturn, nil
}

// ToSegmenter concerts model to a segmenter.
func (m *RegisteredModel) ToSegmenter() (segmentation.Segmenter, error) {
	toReturn, ok := m.Model.(segmentation.Segmenter)
	if !ok {
		return nil, errors.New("couldn't convert model to segmenter")
	}
	return toReturn, nil
}

//  DetectorNames returns a copy of all detector names
func (mm ModelMap) DetectorNames() []string {
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

// ClassifierNames returns copy of all classifier names.
func (mm ModelMap) ClassifierNames() []string {
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

// SegmenterNames returns an copy of all segmenter names.
func (mm ModelMap) SegmenterNames() []string {
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

func (mm ModelMap) getModelType(name string) (VisModelType, error) {
	m, ok := mm[name]
	if !ok {
		return "", errors.Errorf("no such vision model with name %q", name)
	}
	return m.ModelType, nil
}

// ModelLookup checks to see if model is valid.
func (mm ModelMap) ModelLookup(name string) (RegisteredModel, error) {
	m, ok := mm[name]
	if !ok {
		return RegisteredModel{}, errors.Errorf("no such vision model with name %q", name)
	}
	return m, nil
}

// ModelNames returns an array copy of all model names.
func (mm ModelMap) ModelNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		names = append(names, name)
	}
	return names
}

// RemoveVisModel removes models from valid models.
func (mm ModelMap) RemoveVisModel(name string, logger golog.Logger) error {
	if _, ok := mm[name]; !ok {
		logger.Infof("no such vision model with name %s", name)
		return nil
	}

	if mm[name].Closer != nil {
		err := mm[name].Closer.Close()
		if err != nil {
			return err
		}
	}
	delete(mm, name)
	return nil
}

// RegisterVisModel registers a new model.
func (mm ModelMap) RegisterVisModel(name string, m *RegisteredModel, logger golog.Logger) error {
	if m == nil || m.Model == nil {
		return errors.Errorf("cannot register a nil model: %s", name)
	}
	if m.Closer != nil {
		mm[name] = RegisteredModel{
			Model: m.Model, ModelType: m.ModelType,
			Closer: m.Closer, SegParams: m.SegParams,
		}
		return nil
	}
	if _, old := mm[name]; old {
		logger.Infof("overwriting the model with name: %s", name)
	}

	mm[name] = RegisteredModel{
		Model: m.Model, ModelType: m.ModelType,
		Closer: nil, SegParams: m.SegParams,
	}
	return nil
}

// RegisterNewVisModels take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func RegisterNewVisModels(ctx context.Context, mm ModelMap, attrs *Attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerNewVisModels")
	defer span.End()
	for _, attr := range attrs.ModelRegistry {
		logger.Debugf("adding vision model %q of type %q", attr.Name, attr.Type)
		switch VisModelType(attr.Type) { //nolint: exhaustive
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
		// ObjectSegmenters are registered directly from detectors in vision.registerSegmenterFromDetector
		default:
			return newVisModelTypeNotImplemented(attr.Type)
		}
	}
	return nil
}
