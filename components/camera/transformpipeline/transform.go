package transformpipeline

import (
	"context"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
)

// transformType is the list of allowed transforms that can be used in the pipeline.
type transformType string

// the allowed transforms.
const (
	transformTypeUnspecified     = transformType("")
	transformTypeIdentity        = transformType("identity")
	transformTypeRotate          = transformType("rotate")
	transformTypeResize          = transformType("resize")
	transformTypeDepthPretty     = transformType("depth_to_pretty")
	transformTypeOverlay         = transformType("overlay")
	transformTypeUndistort       = transformType("undistort")
	transformTypeDetections      = transformType("detections")
	transformTypeDepthEdges      = transformType("depth_edges")
	transformTypeDepthPreprocess = transformType("depth_preprocess")
)

// Transformation states the type of transformation and the attributes that are specific to the given type.
type Transformation struct {
	Type       string              `json:"type"`
	Attributes config.AttributeMap `json:"attributes"`
}

// buildTransform uses the Transformation config to build the desired transform ImageSource.
func buildTransform(
	ctx context.Context, r robot.Robot, source gostream.VideoSource, stream camera.ImageType, tr Transformation,
) (gostream.VideoSource, camera.ImageType, error) {
	switch transformType(tr.Type) {
	case transformTypeUnspecified, transformTypeIdentity:
		return source, stream, nil
	case transformTypeRotate:
		return newRotateTransform(ctx, source, stream)
	case transformTypeResize:
		return newResizeTransform(ctx, source, stream, tr.Attributes)
	case transformTypeDepthPretty:
		return newDepthToPrettyTransform(ctx, source, stream)
	case transformTypeOverlay:
		return newOverlayTransform(ctx, source, stream, tr.Attributes)
	case transformTypeUndistort:
		return newUndistortTransform(ctx, source, stream, tr.Attributes)
	case transformTypeDetections:
		return newDetectionsTransform(ctx, source, r, tr.Attributes)
	case transformTypeDepthEdges:
		return newDepthEdgesTransform(ctx, source, tr.Attributes)
	case transformTypeDepthPreprocess:
		return newDepthPreprocessTransform(ctx, source)
	default:
		return nil, camera.UnspecifiedStream, errors.Errorf("do not know camera transform of type %q", tr.Type)
	}
}
