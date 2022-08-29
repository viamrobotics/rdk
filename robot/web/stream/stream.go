// Package webstream provides controls for streaming from the web server.
package webstream

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/edaniels/gostream"
	"go.viam.com/utils"

	"go.viam.com/rdk/rlog"
)

// StreamVideoSource starts a stream from a video source with a throttled error handler.
func StreamVideoSource(ctx context.Context, source gostream.VideoSource, stream gostream.Stream, backoffOpts *BackoffTuningOptions) error {
	return gostream.StreamVideoSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler())
}

// StreamAudioSource starts a stream from an audio source with a throttled error handler.
func StreamAudioSource(ctx context.Context, source gostream.AudioSource, stream gostream.Stream, backoffOpts *BackoffTuningOptions) error {
	return gostream.StreamAudioSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler())
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

// GetSleepTimeFromErrorCount returns a sleep time from an error count.
func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errorCount int) time.Duration {
	if errorCount < 1 || opts == nil {
		return 0
	}
	multiplier := math.Pow(2, float64(errorCount-1))
	uncappedSleep := opts.BaseSleep * time.Duration(multiplier)
	sleep := math.Min(float64(uncappedSleep), float64(opts.MaxSleep))
	return time.Duration(sleep)
}

func (opts *BackoffTuningOptions) getErrorThrottledHandler() func(context.Context, error) {
	var prevErr error
	var errorCount int
	lastErrTime := time.Now()

	return func(ctx context.Context, err error) {
		now := time.Now()
		if now.Sub(lastErrTime) > opts.Cooldown {
			errorCount = 0
		}
		lastErrTime = now

		if errors.Is(prevErr, err) {
			errorCount++
		} else {
			prevErr = err
			errorCount = 1
		}

		sleep := opts.GetSleepTimeFromErrorCount(errorCount)
		rlog.Logger.Debugw("error getting media", "error", err, "count", errorCount, "sleep", sleep)
		utils.SelectContextOrWait(ctx, sleep)
	}
}
