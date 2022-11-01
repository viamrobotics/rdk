package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/gostream"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// detectorAttrs is the attribute struct for detectors (their name as found in the vision service).
type detectorAttrs struct {
	DetectorName        string  `json:"detector_name"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
}

// detectorSource takes an image from the camera, and overlays the detections from the detector.
type detectorSource struct {
	stream       gostream.VideoStream
	detectorName string
	confFilter   objectdetection.Postprocessor
	r            robot.Robot
}

func newDetectionsTransform(
	ctx context.Context,
	source gostream.VideoSource, r robot.Robot, am config.AttributeMap,
) (gostream.VideoSource, error) {
	conf, err := config.TransformAttributeMapToStruct(&(detectorAttrs{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*detectorAttrs)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	confFilter := objectdetection.NewScoreFilter(attrs.ConfidenceThreshold)
	detector := &detectorSource{
		gostream.NewEmbeddedVideoStream(source),
		attrs.DetectorName,
		confFilter,
		r,
	}
	return camera.NewFromReader(ctx, detector, nil, camera.ColorStream)
}

// Next returns the image overlaid with the detection bounding boxes.
func (ds *detectorSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::detector::Read")
	defer span.End()
	// get the bounding boxes from the service
	srv, err := vision.FirstFromRobot(ds.r)
	if err != nil {
		return nil, nil, fmt.Errorf("source_detector cant find vision service: %w", err)
	}
	// get image from source camera
	img, release, err := ds.stream.Next(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get next source image: %w", err)
	}
	dets, err := srv.Detections(ctx, img, ds.detectorName, map[string]interface{}{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not get detections: %w", err)
	}
	// overlay detections of the source image
	dets = ds.confFilter(dets)
	res, err := objectdetection.Overlay(img, dets)
	if err != nil {
		return nil, nil, fmt.Errorf("could not overlay bounding boxes: %w", err)
	}
	return res, release, nil
}

func (ds *detectorSource) Close(ctx context.Context) error {
	return ds.stream.Close(ctx)
}
