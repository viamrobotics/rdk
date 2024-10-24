// Package fake implements a fake vision service which always returns the user specified detections/classifications.
package fake

import (
	"context"
	"image"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// Model is the model of the fake buildin camera.
var Model = resource.DefaultModelFamily.WithModel("fake")

const (
	fakeClassLabel = "a_classification"
	fakeClassScore = 0.85
	fakeDetLabel   = "a_detection"
	fakeDetScore   = 0.92
)

func init() {
	resource.RegisterService(vision.API, Model, resource.Registration[vision.Service, *Config]{
		DeprecatedRobotConstructor: func(
			ctx context.Context, r any, c resource.Config, logger logging.Logger,
		) (vision.Service, error) {
			actualR, err := rdkutils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerFake(c.ResourceName(), actualR)
		},
	})
}

func fakeClassifier(ctx context.Context, img image.Image) (classification.Classifications, error) {
	fakeClass := classification.NewClassification(fakeClassScore, fakeClassLabel)
	cls := classification.Classifications{fakeClass}
	return cls, nil
}

func fakeDetector(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
	bounds := img.Bounds()
	boundingBox := image.Rect(
		int(float64(bounds.Max.X)*0.25),
		int(float64(bounds.Max.Y)*0.25),
		int(float64(bounds.Max.X)*0.75),
		int(float64(bounds.Max.Y)*0.75),
	)
	fakeDet := objectdetection.NewDetection(boundingBox, fakeDetScore, fakeDetLabel)
	dets := []objectdetection.Detection{fakeDet}
	return dets, nil
}

// Config are the attributes of the fake vision config.
type Config struct{}

// Validate checks that the config attributes are valid for a fake camera.
func (conf *Config) Validate(path string) ([]string, error) {
	return nil, nil
}

// registerFake creates a new fake vision service from the config.
func registerFake(
	name resource.Name,
	r robot.Robot,
) (vision.Service, error) {
	return vision.NewService(name, r, nil, fakeClassifier, fakeDetector, nil)
}
