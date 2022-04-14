package segmentation

import (
	"context"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/vision"
)

// A Segmenter is a function that takes images/pointclouds from an input camera and segments them into objects.
type Segmenter func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error)
