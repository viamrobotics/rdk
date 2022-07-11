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
// to reduce the number of errors logged in the case of minor dicontinuity
// in the camera stream.
// The number of milliseconds slept at a particular attempt i is determined by
// min(ExpBase^(i) + Offset, MaxSleepMilliSec).
type BackoffTuningOptions struct {
	// ExpBase is a tuning parameter for backoff used as described above
	ExpBase float64
	// Offset is a tuning parameter for backoff used as described above
	Offset float64
	// MaxSleepMilliSec determines the maximum amount of time that streamSource is
	// permitted to a sleep after receiving a single error
	MaxSleepMilliSec float64
	// MaxSleepAttempts determines the number of consecutive errors for which
	// streamSource will sleep
	MaxSleepAttempts int
}

// GetSleepTimeFromErrorCount returns a sleep time from an error count.
func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errCount int) int {
	expBackoffMillisec := math.Pow(opts.ExpBase, float64(errCount)) + opts.Offset
	expBackoffNanosec := expBackoffMillisec * math.Pow10(6)
	maxSleepNanosec := opts.MaxSleepMilliSec * math.Pow10(6)
	return int(math.Min(expBackoffNanosec, maxSleepNanosec))
}

func (opts *BackoffTuningOptions) getErrorThrottledHandler() func(context.Context, error) bool {
	var prevErr error
	errorCount := 0

	return func(ctx context.Context, err error) bool {
		if err != nil {
			switch {
			case prevErr == nil:
				prevErr = err
			case errors.Is(prevErr, err):
				errorCount++
			default:
				errorCount = 0
			}

			canSleep := (errorCount > 0) && (errorCount < opts.MaxSleepAttempts)
			if canSleep && (opts != nil) {
				rlog.Logger.Debugw("error getting frame", "error", err)
				dur := opts.GetSleepTimeFromErrorCount(errorCount)
				utils.SelectContextOrWait(ctx, time.Duration(dur))
			}
			return true
		}
		errorCount = 0
		return false
	}
}
