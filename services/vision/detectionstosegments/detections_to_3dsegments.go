//go:build !no_media

// Package detectionstosegments uses a 2D segmenter and a camera that can project its images
// to 3D to project the bounding boxes to 3D in order to created a segmented point cloud.
package detectionstosegments

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.DefaultModelFamily.WithModel("detector_3d_segmenter")

func init() {
	resource.RegisterService(vision.API, model, resource.Registration[vision.Service, *segmentation.DetectionSegmenterConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (vision.Service, error) {
			attrs, err := resource.NativeConfig[*segmentation.DetectionSegmenterConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return register3DSegmenterFromDetector(ctx, c.ResourceName(), attrs, actualR)
		},
	})
}

// register3DSegmenterFromDetector creates a 3D segmenter from a previously registered detector.
func register3DSegmenterFromDetector(
	ctx context.Context,
	name resource.Name,
	conf *segmentation.DetectionSegmenterConfig,
	r robot.Robot,
) (vision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::register3DSegmenterFromDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for 3D segmenter made from a detector cannot be nil")
	}
	detectorService, err := vision.FromRobot(r, conf.DetectorName)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find necessary dependency, detector %q", conf.DetectorName)
	}
	confThresh := 0.5 // default value
	if conf.ConfidenceThresh > 0.0 {
		confThresh = conf.ConfidenceThresh
	}
	detector := func(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
		return detectorService.Detections(ctx, img, nil)
	}
	segmenter, err := segmentation.DetectionSegmenter(objectdetection.Detector(detector), conf.MeanK, conf.Sigma, confThresh)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create 3D segmenter from detector")
	}
	return vision.NewService(name, r, nil, nil, detector, segmenter)
}
