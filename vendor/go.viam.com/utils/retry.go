package utils

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

// RetryError is emitted by RetryNTimes if all the attempts fail. It unwraps to the last error from retrying.
type RetryError struct {
	inner    error
	attempts int
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("failed after %d retry attempts: %v", e.attempts, e.inner)
}

func (e *RetryError) Unwrap() error {
	return e.inner
}

// RetryNTimesWithSleep will run `fallibleFunc` `retryAttempts` times before failing with the last error it got from the function.
// If `retryableErrors` is supplied, only those errors will be retried.
// It will wait for `retryDelay` between attempts.
func RetryNTimesWithSleep[T any](
	ctx context.Context,
	fallibleFunc func() (T, error),
	retryAttempts int,
	retryDelay time.Duration,
	retryableErrors ...error,
) (T, error) {
	var lastError error
	var emptyT T

	for range retryAttempts {
		val, err := fallibleFunc()
		if err == nil || (len(retryableErrors) != 0 &&
			!slices.ContainsFunc(retryableErrors, func(target error) bool { return errors.Is(err, target) })) {
			return val, err
		}

		lastError = err

		if ctx.Err() != nil {
			return emptyT, ctx.Err()
		}

		select {
		case <-ctx.Done():
			return emptyT, ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	return emptyT, &RetryError{attempts: retryAttempts, inner: lastError}
}

// RetryNTimes will run `fallibleFunc` `retryAttempts` times before failing with the last error it got from the function.
// If `retryableErrors` is supplied, only those errors will be retried.
// It will wait 1 second between attempts. Use RetryNTimesWithSleep to change this.
func RetryNTimes[T any](
	ctx context.Context,
	fallibleFunc func() (T, error),
	retryAttempts int,
	retryableErrors ...error,
) (T, error) {
	return RetryNTimesWithSleep(
		ctx,
		fallibleFunc,
		retryAttempts,
		time.Second,
		retryableErrors...,
	)
}
