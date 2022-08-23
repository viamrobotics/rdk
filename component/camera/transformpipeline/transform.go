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

// the identity transforms. Other transformType names are defined in their respective go files.
const (
	unspecifiedTrasform = transformType("")
	identityTransform   = transformType("identity")
)

// NewUnknownTransformType returns an error of an unknown input transformType.
func NewUnknownTransformType(t string) error {
	return errors.Errorf("do not know camera transform of type %q", t)
}

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
	case unspecifiedTrasform, identityTransform:
		return source, nil
	case rotateTransform:
		return newRotateTransform(source, stream)
	case resizeTransform:
		return newResizeTransform(source, stream, tr.Attributes)
	case depthPrettyTransform:
		return newDepthToPrettyTransform(ctx, source, cfg.AttrConfig)
	case overlayTransform:
		return newOverlayTransform(ctx, source, cfg.AttrConfig)
	case undistortTransform:
		return newUndistortTransform(source, stream, tr.Attributes)
	case detectionsTransform:
		return newDetectionsTransform(source, r, tr.Attributes)
	case depthEdgesTransform:
		return newDepthEdgesTransform(source, tr.Attributes)
	case depthPreprocessTransform:
		return newDepthPreprocessTransform(source)
	default:
		return nil, NewUnknownTransformType(tr.Type)
	}
}
