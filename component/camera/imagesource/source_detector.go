package imagesource

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"detector",
		registry.Component{RobotConstructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*detectorAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			cam, err := camera.FromRobot(r, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera (%s): %w", sourceName, err)
			}
			confFilter := objectdetection.NewScoreFilter(attrs.Confidence)
			detector := &detectorSource{cam, sourceName, attrs.DetectorName, confFilter, r, logger}
			return camera.New(detector, attrs.AttrConfig, cam)
		}})

	config.RegisterComponentAttributeMapConverter(
		camera.SubtypeName,
		"detector",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf detectorAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*detectorAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&detectorAttrs{},
	)
}

// detectorAttrs is the attribute struct for detectors (their name as found in the vision service).
type detectorAttrs struct {
	*camera.AttrConfig
	DetectorName string  `json:"detector_name"`
	Confidence   float64 `json:"confidence"`
}

// detectorSource takes an image from the camera, and overlays the detections from the detector.
type detectorSource struct {
	source       camera.Camera
	cameraName   string
	detectorName string
	confFilter   objectdetection.Postprocessor
	r            robot.Robot
	logger       golog.Logger
}

// Next returns the image overlaid with the detection bounding boxes.
func (ds *detectorSource) Next(ctx context.Context) (image.Image, func(), error) {
	// get the bounding boxes from the service
	srv, err := vision.FromRobot(ds.r)
	if err != nil {
		return nil, nil, fmt.Errorf("source_detector cant find vision service: %w", err)
	}
	dets, err := srv.GetDetections(ctx, ds.cameraName, ds.detectorName)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get detections: %w", err)
	}
	// get image from source camera
	img, release, err := ds.source.Next(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get next source image: %w", err)
	}
	// overlay detections of the source image
	dets = ds.confFilter(dets)
	res, err := objectdetection.Overlay(img, dets)
	if err != nil {
		return nil, nil, fmt.Errorf("could not overlay bounding boxes: %w", err)
	}
	return res, release, nil
}
