package vision

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// DetectorType defines what detector types are known.
type DetectorType string

// The set of allowed detector types.
const (
	TFLiteType     = DetectorType("tflite")
	TensorFlowType = DetectorType("tensorflow")
	ColorType      = DetectorType("color")
)

// newDetectorTypeNotImplemented is used when the detector type is not implemented.
func newDetectorTypeNotImplemented(name string) error {
	return errors.Errorf("detector type %q is not implemented", name)
}

// DetectorConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type DetectorConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// registerNewDetectors take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func registerNewDetectors(ctx context.Context, dm detectorMap, attrs *Attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerNewDetectors")
	defer span.End()
	for _, attr := range attrs.DetectorRegistry {
		logger.Debugf("adding detector %q of type %s", attr.Name, attr.Type)
		switch DetectorType(attr.Type) {
		case TFLiteType:
			return newDetectorTypeNotImplemented(attr.Type)
		case TensorFlowType:
			return newDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(dm, &attr)
			if err != nil {
				return err
			}
		default:
			return newDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

// GetDetectorNames returns a list of the all the names of the detectors in the detector map.
func (vs *visionService) GetDetectorNames(ctx context.Context) ([]string, error) {
	_, span := trace.StartSpan(ctx, "service::vision::GetDetectorNames")
	defer span.End()
	return vs.detReg.detectorNames(), nil
}

// AddDetector adds a new detector from an Attribute config struct.
func (vs *visionService) AddDetector(ctx context.Context, cfg DetectorConfig) error {
	ctx, span := trace.StartSpan(ctx, "service::vision::AddDetector")
	defer span.End()
	attrs := &Attributes{DetectorRegistry: []DetectorConfig{cfg}}
	return registerNewDetectors(ctx, vs.detReg, attrs, vs.logger)
}

// GetDetections returns the detections of the next image from the given camera and the given detector.
func (vs *visionService) GetDetections(ctx context.Context, cameraName, detectorName string) ([]objdet.Detection, error) {
	ctx, span := trace.StartSpan(ctx, "service::vision::GetDetections")
	defer span.End()
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	detector, err := vs.detReg.detectorLookup(detectorName)
	if err != nil {
		return nil, err
	}
	img, release, err := cam.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	return detector(img)
}
