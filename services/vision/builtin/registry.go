//go:build !arm && !windows

package builtin

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

// VisOperation defines what types of operations are allowed by the vision service.
type VisOperation string

// The set of allowed vision model types.
const (
	TFLiteDetector    = vision.VisModelType("tflite_detector")
	TFDetector        = vision.VisModelType("tf_detector")
	ColorDetector     = vision.VisModelType("color_detector")
	TFLiteClassifier  = vision.VisModelType("tflite_classifier")
	TFClassifier      = vision.VisModelType("tf_classifier")
	RCSegmenter       = vision.VisModelType("radius_clustering_segmenter")
	DetectorSegmenter = vision.VisModelType("detector_segmenter")
)

// registeredModelParameterSchemas maps the vision model types to the necessary parameters needed to create them.
var registeredModelParameterSchemas = map[vision.VisModelType]*jsonschema.Schema{
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
var visModelToOpMap = map[vision.VisModelType]VisOperation{
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

type modelMap map[string]registeredModel

// registeredModel struct that holds models parameters.
type registeredModel struct {
	Model     interface{}
	ModelType vision.VisModelType
	Closer    io.Closer
}

// ToDetector converts model to a dectector.
func (m *registeredModel) toDetector() (objectdetection.Detector, error) {
	toReturn, ok := m.Model.(objectdetection.Detector)
	if !ok {
		return nil, errors.New("couldn't convert model to detector")
	}
	return toReturn, nil
}

// ToClassifier converts model to a classifier.
func (m *registeredModel) toClassifier() (classification.Classifier, error) {
	toReturn, ok := m.Model.(classification.Classifier)
	if !ok {
		return nil, errors.New("couldn't convert model to classifier")
	}
	return toReturn, nil
}

// ToSegmenter concerts model to a segmenter.
func (m *registeredModel) toSegmenter() (segmentation.Segmenter, error) {
	toReturn, ok := m.Model.(segmentation.Segmenter)
	if !ok {
		return nil, errors.New("couldn't convert model to segmenter")
	}
	return toReturn, nil
}

// DetectorNames returns list copy of all detector names.
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

// ClassifierNames returns a list copy of all classifier names.
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

// SegmenterNames returns a list copy of all segmenter names.
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

func (mm modelMap) getModelType(name string) (vision.VisModelType, error) {
	m, ok := mm[name]
	if !ok {
		return "", errors.Errorf("no such vision model with name %q", name)
	}
	return m.ModelType, nil
}

// modelLookup checks to see if model is valid.
func (mm modelMap) modelLookup(name string) (registeredModel, error) {
	m, ok := mm[name]
	if !ok {
		return registeredModel{}, errors.Errorf("no such vision model with name %q", name)
	}
	return m, nil
}

// ModelNames returns an array copy of all model names.
func (mm modelMap) ModelNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		names = append(names, name)
	}
	return names
}

// removeVisModel removes models from valid models.
func (mm modelMap) removeVisModel(name string, logger golog.Logger) error {
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
func (mm modelMap) RegisterVisModel(name string, m *registeredModel, logger golog.Logger) error {
	if m == nil || m.Model == nil {
		return errors.Errorf("cannot register a nil model: %s", name)
	}
	if m.Closer != nil {
		mm[name] = registeredModel{
			Model: m.Model, ModelType: m.ModelType, Closer: m.Closer,
		}
		return nil
	}
	if _, old := mm[name]; old {
		logger.Infof("overwriting the model with name: %s", name)
	}

	mm[name] = registeredModel{
		Model: m.Model, ModelType: m.ModelType, Closer: nil,
	}
	return nil
}

// registerNewVisModels take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func registerNewVisModels(ctx context.Context, mm modelMap, attrs *vision.Attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerNewVisModels")
	defer span.End()
	var err error
	for _, attr := range attrs.ModelRegistry {
		logger.Debugf("adding vision model %q of type %q", attr.Name, attr.Type)
		switch vision.VisModelType(attr.Type) {
		case TFLiteDetector:
			multierr.AppendInto(&err, registerTfliteDetector(ctx, mm, &attr, logger))
		case TFLiteClassifier:
			multierr.AppendInto(&err, registerTfliteClassifier(ctx, mm, &attr, logger))
		case TFDetector:
			multierr.AppendInto(&err, newVisModelTypeNotImplemented(attr.Type))
		case TFClassifier:
			multierr.AppendInto(&err, newVisModelTypeNotImplemented(attr.Type))
		case ColorDetector:
			multierr.AppendInto(&err, registerColorDetector(ctx, mm, &attr, logger))
		case RCSegmenter:
			multierr.AppendInto(&err, registerRCSegmenter(ctx, mm, &attr, logger))
		case DetectorSegmenter:
			multierr.AppendInto(&err, registerSegmenterFromDetector(ctx, mm, &attr, logger))
		default:
			multierr.AppendInto(&err, newVisModelTypeNotImplemented(attr.Type))
		}
	}
	return err
}
