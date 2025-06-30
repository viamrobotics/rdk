package vision

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

// DeprecatedNewService wraps the vision model in the struct that fulfills the vision service
// interface. Register this service with DeprecatedRobotConstructor.
func DeprecatedNewService(
	name resource.Name,
	r robot.Robot,
	c func(ctx context.Context) error,
	cf classification.Classifier,
	df objectdetection.Detector,
	s3f segmentation.Segmenter,
	defaultCamera string,
) (Service, error) {
	if cf == nil && df == nil && s3f == nil {
		return nil, errors.Errorf(
			"model %q does not fulfill any method of the vision service. It is neither a detector, nor classifier, nor 3D segmenter", name)
	}

	p := Properties{false, false, false}
	if cf != nil {
		p.ClassificationSupported = true
	}
	if df != nil {
		p.DetectionSupported = true
	}
	if s3f != nil {
		p.ObjectPCDsSupported = true
	}

	logger := r.Logger()

	cameraGetter := func(cameraName string) (camera.Camera, error) {
		return camera.FromRobot(r, cameraName)
	}

	return &vizModel{
		Named:           name.AsNamed(),
		logger:          logger,
		properties:      p,
		closerFunc:      c,
		getCamera:       cameraGetter,
		classifierFunc:  cf,
		detectorFunc:    df,
		segmenter3DFunc: s3f,
		defaultCamera:   defaultCamera,
	}, nil
}
