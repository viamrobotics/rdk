package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	rdkutils "go.viam.com/rdk/utils"
)

type depthEdgesAttrs struct {
	HiThresh   float64 `json:"high_threshold"`
	LoThresh   float64 `json:"low_threshold"`
	BlurRadius float64 `json:"blur_radius"`
}

// depthEdgesSource applies a Canny Edge Detector to the depth map.
type depthEdgesSource struct {
	source     gostream.ImageSource
	detector   *rimage.CannyEdgeDetector
	blurRadius float64
}

func newDepthEdgesTransform(source gostream.ImageSource, am config.AttributeMap) (gostream.ImageSource, error) {
	conf, err := config.TransformAttributeMapToStruct(&(depthEdgesAttrs{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*depthEdgesAttrs)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(attrs.HiThresh, attrs.LoThresh, true)
	return &depthEdgesSource{source, canny, attrs.BlurRadius}, nil
}

// Next applies a canny edge detector on the depth map of the next image.
func (os *depthEdgesSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot transform for depth edges")
	}
	edges, err := os.detector.DetectDepthEdges(dm, os.blurRadius)
	if err != nil {
		return nil, nil, err
	}
	return edges, closer, nil
}
