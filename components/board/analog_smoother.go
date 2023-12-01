package board

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

var errStopReading = errors.New("stop reading")

// An AnalogSmoother smooths the readings out from an underlying reader.
type AnalogSmoother struct {
	Raw                     AnalogReader
	AverageOverMillis       int
	SamplesPerSecond        int
	data                    *utils.RollingAverage
	lastData                int
	lastError               atomic.Pointer[errValue]
	logger                  logging.Logger
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
}

// SmoothAnalogReader wraps the given reader in a smoother.
func SmoothAnalogReader(r AnalogReader, c AnalogReaderConfig, logger logging.Logger) *AnalogSmoother {
	cancelCtx, cancel := context.WithCancel(context.Background())
	smoother := &AnalogSmoother{
		Raw:               r,
		AverageOverMillis: c.AverageOverMillis,
		SamplesPerSecond:  c.SamplesPerSecond,
		logger:            logger,
		cancel:            cancel,
	}
	if smoother.SamplesPerSecond <= 0 {
		logger.Debug("Can't read nonpositive samples per second; defaulting to 1 instead")
		smoother.SamplesPerSecond = 1
	}
	smoother.Start(cancelCtx)
	return smoother
}

// An errValue is used to atomically store an error.
type errValue struct {
	present bool
	err     error
}

// Close stops the smoothing routine.
func (as *AnalogSmoother) Close(ctx context.Context) error {
	as.cancel()
	as.activeBackgroundWorkers.Wait()
	return nil
}

// Read returns the smoothed out reading.
func (as *AnalogSmoother) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	if as.data == nil { // We're using raw data, and not averaging
		return as.lastData, nil
	}

	avg := as.data.Average()
	lastErr := as.lastError.Load()
	if lastErr == nil {
		return avg, nil
	}
	//nolint:forcetypeassert
	if lastErr.present {
		return avg, lastErr.err
	}
	return avg, nil
}

// Start begins the smoothing routine that reads from the underlying
// analog reader.
func (as *AnalogSmoother) Start(ctx context.Context) {
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
	var nanosBetween int
	if numSamples >= 1 {
		as.data = utils.NewRollingAverage(numSamples)
		nanosBetween = 1e9 / as.SamplesPerSecond
	} else {
		as.logger.CDebug(ctx, "Too few samples to smooth over; defaulting to raw data.")
		as.data = nil
		nanosBetween = as.AverageOverMillis * 1e6
	}

	as.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			start := time.Now()
			reading, err := as.Raw.Read(ctx, nil)
			as.lastError.Store(&errValue{err != nil, err})
			if err != nil {
				if errors.Is(err, errStopReading) {
					break
				}
				as.logger.CInfow(ctx, "error reading analog", "error", err)
				continue
			}

			as.lastData = reading
			if as.data != nil {
				as.data.Add(reading)
			}

			end := time.Now()

			toSleep := int64(nanosBetween) - (end.UnixNano() - start.UnixNano())
			if !goutils.SelectContextOrWait(ctx, time.Duration(toSleep)) {
				return
			}
		}
	}, as.activeBackgroundWorkers.Done)
}
