package vision

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
)

// DetectorType defines what detector types are known.
type VisModelType string

// The set of allowed classifier types.
const (
	TFLiteDetector   = VisModelType("tflite_detector")
	TFDetector       = VisModelType("tf_detector")
	ColorDetector    = VisModelType("color_detector")
	TFLiteClassifier = VisModelType("tflite_classifier")
	TFClassifier     = VisModelType("tf_classifier")
	RCSegmenter      = VisModelType("radius_clustering_segmenter")
	ObjectSegmenter  = VisModelType("object_segmenter")
)

var detectorList, classifierList, segmenterList []string

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
	isLoaded  bool
	SegParams []utils.TypedName
}

func (mm modelMap) DetectorNames() []string {
	return detectorList
}

func (mm modelMap) ClassifierNames() []string {
	return classifierList
}

func (mm modelMap) SegmenterNames() []string {
	return segmenterList
}

func (mm modelMap) modelLookup(name string) (registeredModel, error) {
	m, ok := mm[name]
	if !ok {
		return registeredModel{}, errors.Errorf("no such vision model with name %q", name)
	}

	//switch m.modelType {
	//case TFLiteDetector:
	//	return m.model.(objectdetection.Detector), nil
	//case TFDetector:
	//	return m.model.(objectdetection.Detector), nil
	//case ColorDetector:
	//	return m.model.(objectdetection.Detector), nil
	//case TFLiteClassifier:
	//	return m.model.(classification.Classifier), nil
	//case TFClassifier:
	//	return m.model.(classification.Classifier), nil
	//case RadiusClusteringSegmenter:
	//	return m.model.(segmentation.Segmenter), nil
	//case ObjectSegmenter:
	//	return m.model.(segmentation.Segmenter), nil
	//default:
	//	return nil, newVisModelTypeNotImplemented(name)
	//}
	return m, nil
}

func (mm modelMap) getModelType(name string) (VisModelType, error) {
	m, ok := mm[name]
	if !ok {
		return "", errors.Errorf("no such vision model with name %q", name)
	}
	return m.modelType, nil
}

func (mm modelMap) modelNames() []string {
	names := make([]string, 0, len(mm))
	for name := range mm {
		names = append(names, name)
	}
	return names
}

func (mm modelMap) removeModel(name string, logger golog.Logger) error {
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
	detectorList = removeIfInList(detectorList, name)
	segmenterList = removeIfInList(segmenterList, name)
	classifierList = removeIfInList(classifierList, name)
	return nil
}

func (mm modelMap) registerVisModel(name string, m *registeredModel, logger golog.Logger) error {
	if m == nil || m.model == nil {
		return errors.Errorf("cannot register a nil model: %s", name)
	}
	if m.closer != nil {
		mm[name] = registeredModel{
			model: m.model, modelType: m.modelType,
			closer: m.closer, isLoaded: m.isLoaded, SegParams: m.SegParams,
		}
		return nil
	}
	if _, old := mm[name]; old {
		logger.Infof("overwriting the model with name: %s", name)
	} else {
		// Add name to appropriate list (only if not already there)
		switch m.modelType {
		case TFClassifier:
			classifierList = append(classifierList, name)
		case TFLiteClassifier:
			classifierList = append(classifierList, name)
		case TFDetector:
			detectorList = append(detectorList, name)
		case TFLiteDetector:
			detectorList = append(detectorList, name)
		case ColorDetector:
			detectorList = append(detectorList, name)
		case RadiusClusteringSegmenter:
			segmenterList = append(segmenterList, name)
		case ObjectSegmenter:
			segmenterList = append(segmenterList, name)
		default:
			return newVisModelTypeNotImplemented(name)
		}
	}
	mm[name] = registeredModel{
		model: m.model, modelType: m.modelType,
		closer: nil, isLoaded: m.isLoaded, SegParams: m.SegParams,
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
		case RadiusClusteringSegmenter:
			return registerRCSegmenter(ctx, mm, &attr, logger)
		default:
			return newVisModelTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

func removeIfInList(list []string, elem string) []string {
	for i, e := range list {
		if e == elem {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}
