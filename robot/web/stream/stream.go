//go:build !no_cgo || android

// Package webstream provides controls for streaming from the web server.
package webstream

import (
	"context"
	"errors"
	"time"

	"go.viam.com/utils"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
)

// StreamVideoSource starts a stream from a video source with a throttled error handler.
func StreamVideoSource(
	ctx context.Context,
	source gostream.VideoSource,
	stream gostream.Stream,
	backoffOpts *BackoffTuningOptions,
	logger logging.Logger,
) error {
	return gostream.StreamVideoSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler(logger, stream.Name()), logger)
}

// StreamAudioSource starts a stream from an audio source with a throttled error handler.
func StreamAudioSource(
	ctx context.Context,
	source gostream.AudioSource,
	stream gostream.Stream,
	backoffOpts *BackoffTuningOptions,
	logger logging.Logger,
) error {
	return gostream.StreamAudioSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler(logger, "audio"), logger)
}

// BackoffTuningOptions represents a set of parameters for determining exponential
// backoff when receiving multiple simultaneous errors.
//
// BaseSleep is the duration to wait after receiving a new error. After that, the wait
// time doubles for every subsequent, consecutive error of the same type, until the wait
// duration reaches the MaxSleep duration.
type BackoffTuningOptions struct {
	// BaseSleep sets the initial amount of time to wait after an error.
	BaseSleep time.Duration
	// MaxSleep determines the maximum amount of time that streamSource is
	// permitted to a sleep after receiving a single error.
	MaxSleep time.Duration
	// Cooldown sets how long since the last error that we can reset our backoff. This
	// should be greater than MaxSleep. This prevents a scenario where we haven't made
	// a call to read for a long time and the error may go away sooner.
	Cooldown time.Duration
}

// Dan: This fixes the backoff bugs where we'd overflow the sleep to a negative value. But this no
// longer obeys the input tuning options. I'm considering that deprecated and to be removed.
var backoffSleeps = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second}

// GetSleepTimeFromErrorCount returns a sleep time from an error count.
func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errorCount int) time.Duration {
	if errorCount < 1 || opts == nil {
		return 0
	}

	errorCount--
	if errorCount >= len(backoffSleeps) {
		return backoffSleeps[len(backoffSleeps)-1]
	}

	return backoffSleeps[errorCount]
}

func (opts *BackoffTuningOptions) getErrorThrottledHandler(logger logging.Logger, streamName string) func(context.Context, error) {
	var prevErr error
	var errorCount int
	lastErrTime := time.Now()

	return func(ctx context.Context, err error) {
		now := time.Now()
		if now.Sub(lastErrTime) > opts.Cooldown {
			errorCount = 0
		}
		lastErrTime = now

		switch {
		case errors.Is(prevErr, err):
			errorCount++
		case prevErr != nil && prevErr.Error() == err.Error():
			errorCount++
		default:
			prevErr = err
			errorCount = 1
		}

		sleep := opts.GetSleepTimeFromErrorCount(errorCount)
		logger.Errorw("error getting media", "streamName", streamName, "error", err, "count", errorCount, "sleep", sleep)

		utils.SelectContextOrWait(ctx, sleep)
	}
}
