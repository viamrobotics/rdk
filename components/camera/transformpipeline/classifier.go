package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/gostream"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

// classifierAttrs is the attribute struct for classifiers (their name as found in the vision service).
type classifierAttrs struct {
	ClassifierName      string  `json:"classifier_name"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
}

// classifierSource takes an image from the camera, and overlays the labels from the classifier.
type classifierSource struct {
	stream         gostream.VideoStream
	classifierName string
	// TODO: create this type
	confFilter classification.Postprocessor
	r          robot.Robot
}

func newClassificationsTransform(
	ctx context.Context,
	source gostream.VideoSource, r robot.Robot, am config.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := config.TransformAttributeMapToStruct(&(classifierAttrs{}), am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	attrs, ok := conf.(*classifierAttrs)
	if !ok {
		return nil, camera.UnspecifiedStream, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}

	// TODO: not really sure what's going on between here and where we construct the confidence
	// filter
	props, err := propsFromVideoSource(ctx, source)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

	if props.DistortionParams != nil {
		cameraModel.Distortion = props.DistortionParams
	}
	confFilter := objectdetection.NewScoreFilter(attrs.ConfidenceThreshold)
	classifier := &classifierSource{
		gostream.NewEmbeddedVideoStream(source),
		attrs.ClassifierName,
		confFilter,
		r,
	}
	cam, err := camera.NewFromReader(ctx, classifier, &cameraModel, camera.ColorStream)
	return cam, camera.ColorStream, err
}

// Read returns the image overlaid with the labels from the classification.
func (cs *classifierSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::classifier::Read")
	defer span.End()
	// get the bounding boxes from the service
	srv, err := vision.FirstFromRobot(cs.r)
	if err != nil {
		return nil, nil, fmt.Errorf("source_detector cant find vision service: %w", err)
	}
	// get image from source camera
	img, release, err := cs.stream.Next(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get next source image: %w", err)
	}
	// TODO: reconsider number of classifications
	numClassifications := 1
	classifications, err := srv.Classifications(ctx, img, cs.classifierName, numClassifications, map[string]interface{}{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not get classifications: %w", err)
	}
	// overlay labels on the source image
	classifications = cs.confFilter(classifications)
	// TODO: write this function
	res, err := classification.Overlay(img, classifications)
	if err != nil {
		return nil, nil, fmt.Errorf("could not overlay labels: %w", err)
	}
	return res, release, nil
}
