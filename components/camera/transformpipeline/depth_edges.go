//go:build !no_media

package transformpipeline

import (
	"context"
	"image"

	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

type depthEdgesConfig struct {
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

func newDepthEdgesTransform(ctx context.Context, source gostream.VideoSource, am utils.AttributeMap,
) (gostream.VideoSource, camera.ImageType, error) {
	conf, err := resource.TransformAttributeMap[*depthEdgesConfig](am)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
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
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(conf.HiThresh, conf.LoThresh, true)
	videoSrc := &depthEdgesSource{gostream.NewEmbeddedVideoStream(source), canny, 3.0}
	src, err := camera.NewVideoSourceFromReader(ctx, videoSrc, &cameraModel, camera.DepthStream)
	if err != nil {
		return nil, camera.UnspecifiedStream, err
	}
	return src, camera.DepthStream, err
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
