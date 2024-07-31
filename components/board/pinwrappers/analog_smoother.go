// Package pinwrappers implements interfaces that wrap the basic board interface and return types,
// and expands them with new methods and interfaces for the built in board models. Current expands
// analog reader and digital interrupt.
package pinwrappers

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

var errStopReading = errors.New("stop reading")

// An AnalogSmoother smooths the readings out from an underlying reader.
type AnalogSmoother struct {
	Raw               board.Analog
	AverageOverMillis int
	SamplesPerSecond  int
	data              *utils.RollingAverage
	lastData          int
	lastError         atomic.Pointer[errValue]
	logger            logging.Logger
	workers           utils.StoppableWorkers
	analogVal         board.AnalogValue
}

// SmoothAnalogReader wraps the given reader in a smoother.
func SmoothAnalogReader(r board.Analog, c board.AnalogReaderConfig, logger logging.Logger) *AnalogSmoother {
	smoother := &AnalogSmoother{
		Raw:               r,
		AverageOverMillis: c.AverageOverMillis,
		SamplesPerSecond:  c.SamplesPerSecond,
		logger:            logger,
	}
	if smoother.SamplesPerSecond <= 0 {
		logger.Debug("Can't read nonpositive samples per second; defaulting to 1 instead")
		smoother.SamplesPerSecond = 1
	}

	// Store the analog reader info
	analogVal, err := smoother.Raw.Read(context.Background(), nil)
	smoother.lastError.Store(&errValue{err != nil, err})
	smoother.analogVal = analogVal

	smoother.Start()
	return smoother
}

// An errValue is used to atomically store an error.
type errValue struct {
	present bool
	err     error
}

// Close stops the smoothing routine.
func (as *AnalogSmoother) Close(ctx context.Context) error {
	as.workers.Stop()
	return nil
}

// Read returns the smoothed out reading.
func (as *AnalogSmoother) Read(ctx context.Context, extra map[string]interface{}) (board.AnalogValue, error) {
	analogVal := board.AnalogValue{
		Min:      as.analogVal.Min,
		Max:      as.analogVal.Max,
		StepSize: as.analogVal.StepSize,
	}

	if as.data == nil { // We're using raw data, and not averaging
		analogVal.Value = as.lastData
		return analogVal, nil
	}
	avg := as.data.Average()
	lastErr := as.lastError.Load()
	analogVal.Value = avg
	if lastErr == nil {
		return analogVal, nil
	}
	//nolint:forcetypeassert
	if lastErr.present {
		return analogVal, lastErr.err
	}
	return analogVal, nil
}

// Start begins the smoothing routine that reads from the underlying
// analog reader.
func (as *AnalogSmoother) Start() {
	// examples 1
	//    AverageOverMillis 10
	//    SamplesPerSecond  1000
	//    numSamples        10

	// examples 2
	//    AverageOverMillis 10
	//    SamplesPerSecond  10000
	//    numSamples        100

	// examples 3
	//    AverageOverMillis 2000
	//    SamplesPerSecond  2
	//    numSamples        4

	numSamples := (as.SamplesPerSecond * as.AverageOverMillis) / 1000
	nanosBetween := 1e9 / as.SamplesPerSecond
	if numSamples >= 1 {
		as.data = utils.NewRollingAverage(numSamples)
	} else {
		as.logger.Debug("Too few samples to smooth over; defaulting to raw data.")
		as.data = nil
	}

	as.workers = utils.NewStoppableWorkers(func(ctx context.Context) {
		consecutiveErrors := 0
		var lastError error

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			start := time.Now()
			reading, err := as.Raw.Read(ctx, nil)
			as.lastError.Store(&errValue{err != nil, err})
			if err == nil {
				as.lastData = reading.Value
				if as.data != nil {
					as.data.Add(reading.Value)
				}
				consecutiveErrors = 0
			} else { // Non-nil error
				if errors.Is(err, errStopReading) {
					break
				}
				if lastError != nil && err.Error() == lastError.Error() {
					consecutiveErrors++
				} else {
					as.logger.CInfow(ctx, "error reading analog", "error", err)
					consecutiveErrors = 0
				}
				// Don't spam the errors: only remind us of the problem every 10 seconds.
				if consecutiveErrors == (as.SamplesPerSecond * 10) {
					as.logger.CErrorw(ctx, "unable to read analog for 10 seconds", "error", err)
					consecutiveErrors = 0
				}
			}
			lastError = err

			end := time.Now()

			toSleep := int64(nanosBetween) - (end.UnixNano() - start.UnixNano())
			if !goutils.SelectContextOrWait(ctx, time.Duration(toSleep)) {
				return
			}
		}
	})
}

func (as *AnalogSmoother) Write(ctx context.Context, value int, extra map[string]interface{}) error {
	return grpc.UnimplementedError
}
