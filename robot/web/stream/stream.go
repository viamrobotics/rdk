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

// BackoffTuningOptions represents a set of parameters for determining
// exponential backoff when receiving multiple simultaneous errors. This is
// to reduce the number of errors logged in the case of minor discontinuity
// in the camera stream.
// The number of milliseconds slept at a particular attempt i is determined by
// min(ExpBase^(i) + Offset, MaxSleepMilliSec).
type BackoffTuningOptions struct {
	// The initial amount of time to wait after an error.
	BaseSleep time.Duration
	// MaxSleep determines the maximum amount of time that streamSource is
	// permitted to a sleep after receiving a single error
	MaxSleep time.Duration
	// MaxSleepAttempts determines the number of consecutive errors for which
	// streamSource will sleep
	MaxSleepAttempts int
}

// GetSleepTimeFromErrorCount returns a sleep time from an error count.
func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errCount int) time.Duration {
	if errCount < 1 {
		return time.Duration(0)
	}
	multiplier := math.Pow(2, float64(errCount-1))
	uncappedSleepNanosec := float64(opts.BaseSleep.Nanoseconds()) * multiplier
	sleepNanosec := math.Min(uncappedSleepNanosec, float64(opts.MaxSleep.Nanoseconds()))
	return time.Duration(int64(sleepNanosec))
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

			if errorCount <= opts.MaxSleepAttempts {
				rlog.Logger.Debugw("error getting frame", "error", err)
				sleep := opts.GetSleepTimeFromErrorCount(errorCount)
				utils.SelectContextOrWait(ctx, sleep)
			} else {
				panic(err)
			}
			return true
		}
		errorCount = 0
		return false
	}
}
