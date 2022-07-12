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

// StreamSource starts a stream from an image source with a throttled error handler.
func StreamSource(ctx context.Context, source gostream.ImageSource, stream gostream.Stream, backoffOpts *BackoffTuningOptions) {
	gostream.StreamSourceWithErrorHandler(ctx, source, stream, backoffOpts.getErrorThrottledHandler())
}

// BackoffTuningOptions represents a set of parameters for determining exponential
// backoff when receiving multiple simultaneous errors.
//
// `BaseSleep` is the duration to wait after receiving a new error. After that, the wait
// time doubles for every subsequent, consecutive error of the same type, until the wait
// duration reaches the `MaxSleep` duration.
type BackoffTuningOptions struct {
	// `BaseSleep` sets the initial amount of time to wait after an error.
	BaseSleep time.Duration
	// MaxSleep determines the maximum amount of time that streamSource is
	// permitted to a sleep after receiving a single error.
	MaxSleep time.Duration
}

// GetSleepTimeFromErrorCount returns a sleep time from an error count.
func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errorCount int) time.Duration {
	if errorCount < 1 {
		return time.Duration(0)
	}
	multiplier := math.Pow(2, float64(errorCount-1))
	uncappedSleep := opts.BaseSleep * time.Duration(multiplier)
	sleep := math.Min(float64(uncappedSleep), float64(opts.MaxSleep))
	return time.Duration(sleep)
}

func (opts *BackoffTuningOptions) getErrorThrottledHandler() func(context.Context, error) bool {
	var prevErr error
	errorCount := 0

	return func(ctx context.Context, err error) bool {
		if err != nil {
			if errors.Is(prevErr, err) {
				errorCount++
			} else {
				prevErr = err
				errorCount = 1
			}

			sleep := opts.GetSleepTimeFromErrorCount(errorCount)
			rlog.Logger.Debugw("error getting frame", "error", err)
			utils.SelectContextOrWait(ctx, sleep)

			return true
		}
		errorCount = 0
		return false
	}
}
