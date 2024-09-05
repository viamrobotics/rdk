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
	return exponentialRetry{
		ctx:    ctx,
		clock:  clock,
		logger: logger,
		name:   name,
		fun:    fun,
	}
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
func (e exponentialRetry) run() error {
	err := e.fun(e.ctx)

	// If no error, return nil for success
	if err == nil {
		e.logger.Debugf("succeeded: %s", e.name)
		return nil
	}

	// Don't retry non-retryable errors.
	if terminalError(err) {
		e.logger.Warnf("hit non retryable error: %v", err)
		return err
	}

	// If the context was cancelled
	// return the error without logging to not spam
	if errors.Is(err, context.Canceled) {
		return err
	}
	e.logger.Infof("entering exponential backoff retry due to retryable error: %v", err)

	// First call failed, so begin exponentialRetry with a factor of RetryExponentialFactor
	nextWait := time.Millisecond * time.Duration(InitialWaitTimeMillis)
	ticker := e.clock.Ticker(nextWait)
	defer ticker.Stop()
	for {
		if err := e.ctx.Err(); err != nil {
			return err
		}

		select {
		case <-e.ctx.Done():
			return e.ctx.Err()
		case <-ticker.C:
			err := e.fun(e.ctx)

			// If no error, return nil for success
			if err == nil {
				e.logger.Debugf("succeeded: %s", e.name)
				return nil
			}

			// If the context was cancelled
			// return the error without logging to not spam
			if errors.Is(err, context.Canceled) {
				return err
			}

			// Don't retry terminal errors.
			if terminalError(err) {
				e.logger.Warnf("hit non retryable error: %v", err)
				return err
			}

			// Otherwise, try again after nextWait.
			offline := isOfflineGRPCError(err)
			nextWait = getNextWait(nextWait, offline)
			status := "online"
			if offline {
				status = "offline"
			}
			e.logger.Infof("hit retryable error that "+
				"indicates connectivity status is: %s "+
				"continuing exponential backoff retry, "+
				"will retry in: %s, error: %v", status, nextWait, err)
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
