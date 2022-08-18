package transformpipeline

import (
	"context"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
)

// transformType is the list of allowed transforms that can be used in the pipeline.
type transformType string

// the allowed types of transforms.
const (
	unspecifiedTrasform      = transformType("")
	identityTransform        = transformType("identity")
	rotateTransform          = transformType("rotate")
	resizeTransform          = transformType("resize")
	depthPrettyTransform     = transformType("depth_to_pretty")
	overlayTransform         = transformType("overlay")
	undistortTrasform        = transformType("undistort")
	detectionsTransform      = transformType("detections")
	depthPreprocessTransform = transformType("depth_preprocess")
	depthEdgesTransform      = transformType("depth_edges")
)

func NewUnknownTransformType(t string) error {
	return errors.Errorf("do not know camera transform of type %q", t)
}

// Transformation states the type of transformation and the attributes that are specific to the given type.
type Transformation struct {
	Type       string              `json:"type"`
	Attributes config.AttributeMap `json:"attributes"`
}

// buildTransform uses the Transformation config to build the desired transform ImageSource
func buildTransform(
	ctx context.Context, source gostream.ImageSource, stream camera.StreamType, tr Transformation,
) (gostream.ImageSource, error) {
	switch transformType(tr.Type) {
	case unspecifiedTrasform, identityTransform:
		return source, nil
	case rotateTransform:
		return newRotateTransform(ctx, source, stream)
	case resizeTransform:
		return newResizeTransform(ctx, source, stream, tr.Attributes)
	default:
		return nil, NewUnknownTransformType(tr.Type)
	}
}
