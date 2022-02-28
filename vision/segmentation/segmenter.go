package segmentation

import (
	"context"

	"github.com/pkg/errors"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// A Segmenter is a function that takes images/pointclouds from an input camera and segments them into objects.
type Segmenter func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error)

// GetSegmenter queries the registery of segmenters to look for a Segmenter by name
func GetSegmenter(ctx context.Context, segmenterName string) (Segmenter, error) {
	segmenterConstructor := registry.SegmenterLookup(segmenterName)
	if segmenterConstructor == nil {
		return nil, errors.Errorf("cannot find segmenter %q in registry", segmenterName)
	}
	result, err := segmenterConstructor.Constructor(ctx)
	if err != nil {
		return nil, err
	}
	segmenter, ok := result.(Segmenter)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(segmenter, result)
	}
}
