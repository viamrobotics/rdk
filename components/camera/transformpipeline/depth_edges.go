package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	rdkutils "go.viam.com/rdk/utils"
)

type depthEdgesAttrs struct {
	HiThresh   float64 `json:"high_threshold_pct"`
	LoThresh   float64 `json:"low_threshold_pct"`
	BlurRadius float64 `json:"blur_radius_px"`
}

// depthEdgesSource applies a Canny Edge Detector to the depth map.
type depthEdgesSource struct {
	stream     gostream.VideoStream
	detector   *rimage.CannyEdgeDetector
	blurRadius float64
}

func newDepthEdgesTransform(ctx context.Context, source gostream.VideoSource, am config.AttributeMap) (gostream.VideoSource, error) {
	conf, err := config.TransformAttributeMapToStruct(&(depthEdgesAttrs{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*depthEdgesAttrs)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(attrs.HiThresh, attrs.LoThresh, true)
	videoSrc := &depthEdgesSource{gostream.NewEmbeddedVideoStream(source), canny, 3.0}
	return camera.NewFromReader(ctx, videoSrc, nil, camera.DepthStream)
}

// Next applies a canny edge detector on the depth map of the next image.
func (os *depthEdgesSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::depthEdges::Read")
	defer span.End()
	i, closer, err := os.stream.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(ctx, i)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot transform for depth edges")
	}
	edges, err := os.detector.DetectDepthEdges(dm, os.blurRadius)
	if err != nil {
		return nil, nil, err
	}
	return edges, closer, nil
}

func (os *depthEdgesSource) Close(ctx context.Context) error {
	return os.stream.Close(ctx)
}
