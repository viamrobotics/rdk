package utils

import (
	"context"
	"time"

	"go.viam.com/rdk/logging"
)

// SlowStartupLogger starts a goroutine that logs every few seconds as long as the context has not timed out or was not cancelled.
func SlowStartupLogger(ctx context.Context, msg, fieldName, fieldVal string, logger logging.Logger) func() {
	slowTicker := time.NewTicker(2 * time.Second)
	firstTick := true

	ctxWithCancel, cancel := context.WithCancel(ctx)
	startTime := time.Now()
	go func() {
		for {
			select {
			case <-slowTicker.C:
				elapsed := time.Since(startTime).Seconds()
				logger.CWarnw(ctx, msg, fieldName, fieldVal, "time elapsed", elapsed)
				if firstTick {
					slowTicker.Reset(3 * time.Second)
					firstTick = false
				} else {
					slowTicker.Reset(5 * time.Second)
				}
			case <-ctxWithCancel.Done():
				return
			}
		}
	}()
	return func() { slowTicker.Stop(); cancel() }
}
