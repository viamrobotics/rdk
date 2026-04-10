//go:build no_cgo && !android

// Package webstream provides controls for streaming from the web server.
package webstream

import (
	"context"

	"github.com/pkg/errors"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
)

// streamVideoSource starts a stream from a video source with a throttled error handler.
func streamVideoSource(
	ctx context.Context,
	source gostream.VideoSource,
	stream gostream.Stream,
	backoffOpts *BackoffTuningOptions,
	logger logging.Logger,
) error {
	return errors.New("not implemented for non-cgo")
}
