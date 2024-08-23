package sync

import (
	"context"
	"errors"
	"time"

	"github.com/benbjohnson/clock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	// public for tests.
	InitialWaitTimeMillis = 200
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	// public for tests.
	RetryExponentialFactor = 2
)

const (
	// offlineWaitTimeSeconds defines the amount of time to wait to retry if the machine is offline.
	offlineWaitTimeSeconds = 60
	maxRetryInterval       = time.Hour
)

func newExponentialRetry(
	ctx context.Context,
	clock clock.Clock,
	logger logging.Logger,
	name string,
	fun func(context.Context) error,
) exponentialRetry {
	return exponentialRetry{ctx: ctx, clock: clock, logger: logger, name: name, fun: fun}
}

type exponentialRetry struct {
	ctx    context.Context
	clock  clock.Clock
	logger logging.Logger
	name   string
	fun    func(context.Context) error
}

// run calls fn and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
// returns nil if completed successfully
// returns context.Cancelled if ctx is cancelled
// all other errors are due to an unrecoverable error.
func (er exponentialRetry) run() error {
	err := er.fun(er.ctx)
	switch {
	// If no error, return nil for success
	case err == nil:
		er.logger.Debugf("exponentialRetry.run %s succeeded", er.name)
		return nil
	// Don't retry non-retryable errors.
	case terminalError(err):
		er.logger.Debugf("exponentialRetry.run %s hit non retryable error: %s", er.name, err.Error())
		return err
	default:
		er.logger.Debugf("exponentialRetry.run: %s entering exponential backoff retry due to retryable error: %s", er.name, err.Error())
	}

	// First call failed, so begin exponentialRetry with a factor of RetryExponentialFactor
	nextWait := time.Millisecond * time.Duration(InitialWaitTimeMillis)
	ticker := er.clock.Ticker(nextWait)
	defer ticker.Stop()
	for {
		if err := er.ctx.Err(); err != nil {
			return err
		}

		select {
		case <-er.ctx.Done():
			return er.ctx.Err()
		case <-ticker.C:
			err := er.fun(er.ctx)
			switch {
			// If no error, return nil for success
			case err == nil:
				er.logger.Debugf("exponentialRetry.run %s succeeded", er.name)
				return nil
			// Don't retry terminal errors.
			case terminalError(err):
				er.logger.Debugf("exponentialRetry.run %s hit non retryable error: %s", er.name, err.Error())
				return err
			}

			// Otherwise, try again after nextWait.
			if !errors.Is(err, context.Canceled) {
				er.logger.Error(err.Error())
			}

			offline := isOfflineGRPCError(err)
			nextWait = getNextWait(nextWait, offline)
			status := "online"
			if offline {
				status = "offline"
			}
			er.logger.Debugf("exponentialRetry.run %s hit transient error, will retry in: %s, "+
				"error indicates connectivity status is %s", er.name, nextWait, status)
			ticker.Reset(nextWait)
		}
	}
}

func isOfflineGRPCError(err error) bool {
	errStatus := status.Convert(err)
	return errStatus.Code() == codes.Unavailable
}

func getNextWait(lastWait time.Duration, isOffline bool) time.Duration {
	if lastWait == time.Duration(0) {
		return time.Millisecond * time.Duration(InitialWaitTimeMillis)
	}

	if isOffline {
		return time.Second * time.Duration(offlineWaitTimeSeconds)
	}

	nextWait := lastWait * time.Duration(RetryExponentialFactor)
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}

// terminalError returns true if retrying will never succeed so that
// the data gets moved to the corrupted data directory and false otherwise.
func terminalError(err error) bool {
	errStatus := status.Convert(err)
	return errStatus.Code() == codes.InvalidArgument || errors.Is(err, proto.Error)
}
