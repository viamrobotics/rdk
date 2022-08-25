package transformpipeline

import (
	"context"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
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
	ctx context.Context, r robot.Robot, source gostream.ImageSource, cfg *transformConfig, tr Transformation,
) (gostream.ImageSource, error) {
	stream := camera.StreamType(cfg.Stream)
	switch transformType(tr.Type) {
	case transformTypeUnspecified, transformTypeIdentity:
		return source, nil
	case transformTypeRotate:
		return newRotateTransform(source, stream)
	case transformTypeResize:
		return newResizeTransform(source, stream, tr.Attributes)
	case transformTypeDepthPretty:
		return newDepthToPrettyTransform(ctx, source, cfg.AttrConfig)
	case transformTypeOverlay:
		return newOverlayTransform(ctx, source, cfg.AttrConfig)
	case transformTypeUndistort:
		return newUndistortTransform(source, stream, tr.Attributes)
	case transformTypeDetections:
		return newDetectionsTransform(source, r, tr.Attributes)
	case transformTypeDepthEdges:
		return newDepthEdgesTransform(source, tr.Attributes)
	case transformTypeDepthPreprocess:
		return newDepthPreprocessTransform(source)
	default:
		return nil, errors.Errorf("do not know camera transform of type %q", tr.Type)
	}
}
