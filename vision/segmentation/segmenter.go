//go:build !no_media

package segmentation

import (
	"context"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/vision"
)

// A Segmenter is a function that takes images/pointclouds from an input source and segments them into objects.
type Segmenter func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error)
