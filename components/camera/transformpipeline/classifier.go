package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/vision"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/classification"
)

// classifierAttrs is the attribute struct for classifiers.
type classifierAttrs struct {
	ClassifierName      string  `json:"classifier_name"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	MaxClassifications  uint32  `json:"max_classifications"`
}

// classifierSource takes an image from the camera, and overlays labels from the classifier.
type classifierSource struct {
	stream             gostream.VideoStream
	classifierName     string
	maxClassifications uint32
	confFilter         classification.Postprocessor
	r                  robot.Robot
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

	props, err := propsFromVideoSource(ctx, source)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

	if props.DistortionParams != nil {
		cameraModel.Distortion = props.DistortionParams
	}
	confFilter := classification.NewScoreFilter(attrs.ConfidenceThreshold)
	var maxClassifications uint32 = 1
	if attrs.MaxClassifications != 0 {
		maxClassifications = attrs.MaxClassifications
	}
	classifier := &classifierSource{
		gostream.NewEmbeddedVideoStream(source),
		attrs.ClassifierName,
		maxClassifications,
		confFilter,
		r,
	}
	cam, err := camera.NewFromReader(ctx, classifier, &cameraModel, camera.ColorStream)
	return cam, camera.ColorStream, err
}

// Read returns the image overlaid with at most max_classifications labels from the classification.
// It overlays the labels along with the confidence scores, as long as the scores are above the
// confidence threshold.
func (cs *classifierSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::classifier::Read")
	defer span.End()

	srv, err := vision.FromRobot(cs.r, cs.classifierName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "source_classifier can't find vision service")
	}
	// get image from source camera
	img, release, err := cs.stream.Next(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get next source image")
	}
	classifications, err := srv.Classifications(ctx, img, int(cs.maxClassifications), map[string]interface{}{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get classifications")
	}
	// overlay labels on the source image
	classifications = cs.confFilter(classifications)
	res, err := classification.Overlay(img, classifications)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not overlay labels")
	}
	return res, release, nil
}

func (cs *classifierSource) Close(ctx context.Context) error {
	return cs.stream.Close(ctx)
}
