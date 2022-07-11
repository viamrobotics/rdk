// Package provides controls for streaming from the web server.
package webstream

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/edaniels/gostream"
	"go.viam.com/rdk/rlog"
	"go.viam.com/utils"
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
// min(ExpBase^(i) + Offset, MaxSleepMilliSec)
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

func (opts *BackoffTuningOptions) GetSleepTimeFromErrorCount(errCount int) int {
	expBackoffMillisec := math.Pow(opts.ExpBase, float64(errCount)) + opts.Offset
	expBackoffNanosec := expBackoffMillisec * math.Pow10(6)
	maxSleepNanosec := opts.MaxSleepMilliSec * math.Pow10(6)
	return int(math.Min(expBackoffNanosec, maxSleepNanosec))
}

func (backoffOpts *BackoffTuningOptions) getErrorThrottledHandler() func(context.Context, error) bool {
	var prevErr error
	errorCount := 0

	return func(ctx context.Context, err error) bool {
		if err != nil {
			if prevErr == nil {
				prevErr = err
			} else if errors.Is(prevErr, err) {
				errorCount += 1
			} else {
				errorCount = 0
			}
			canSleep := (errorCount > 0) && (errorCount < backoffOpts.MaxSleepAttempts)
			if canSleep && (backoffOpts != nil) {
				rlog.Logger.Debugw("error getting frame", "error", err)
				dur := backoffOpts.GetSleepTimeFromErrorCount(errorCount)
				utils.SelectContextOrWait(ctx, time.Duration(dur))
			}
			return true
		} else {
			errorCount = 0
			return false
		}
	}
}
