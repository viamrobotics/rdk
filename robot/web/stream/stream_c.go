//go:build !no_cgo || android

// Package webstream provides controls for streaming from the web server.
package webstream

import (
	"context"

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
	return gostream.StreamVideoSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler(logger, stream.Name()), logger)
}

// streamAudioSource starts a stream from an audio source with a throttled error handler.
func streamAudioSource(
	ctx context.Context,
	source gostream.AudioSource,
	stream gostream.Stream,
	backoffOpts *BackoffTuningOptions,
	logger logging.Logger,
) error {
	return gostream.StreamAudioSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler(logger, "audio"), logger)
}
