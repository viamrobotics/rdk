package segmentation

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/registry"
)

func TestGetSegmenter(t *testing.T) {
	// segmenter that does not exist
	_, err := GetSegmenter(context.Background(), "does_not_exist")
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find segmenter")
	// segmenter has bad constructor
	registry.RegisterSegmenter("error_constructor", registry.Segmenter{
		Constructor: func(ctx context.Context) (interface{}, error) { return nil, errors.New("constructor error") },
	})
	_, err = GetSegmenter(context.Background(), "error_constructor")
	test.That(t, err.Error(), test.ShouldContainSubstring, "constructor error")
	// returned function is not a segmenter
	registry.RegisterSegmenter("not_segmenter", registry.Segmenter{
		Constructor: func(ctx context.Context) (interface{}, error) { return 5, nil },
	})
	_, err = GetSegmenter(context.Background(), "not_segmenter")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected segmentation.Segmenter but got int")
	// success
	_, err = GetSegmenter(context.Background(), RadiusClusteringSegmenter)
	test.That(t, err, test.ShouldBeNil)
}
