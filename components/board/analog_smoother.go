package board

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/utils"
)

var errStopReading = errors.New("stop reading")

// An AnalogSmoother smooths the readings out from an underlying reader.
type AnalogSmoother struct {
	Raw                     AnalogReader
	AverageOverMillis       int
	SamplesPerSecond        int
	data                    *utils.RollingAverage
	lastError               atomic.Value // errValue
	logger                  golog.Logger
	cancel                  func()
	activeBackgroundWorkers *sync.WaitGroup
}

// SmoothAnalogReader wraps the given reader in a smoother.
func SmoothAnalogReader(r AnalogReader, c AnalogConfig, logger golog.Logger) AnalogReader {
	if c.AverageOverMillis <= 0 {
		return r
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	smoother := &AnalogSmoother{
		Raw:                     r,
		AverageOverMillis:       c.AverageOverMillis,
		SamplesPerSecond:        c.SamplesPerSecond,
		logger:                  logger,
		cancel:                  cancel,
		activeBackgroundWorkers: &sync.WaitGroup{},
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
func (as *AnalogSmoother) Close() {
	as.cancel()
	as.activeBackgroundWorkers.Wait()
}

// Read returns the smoothed out reading.
func (as *AnalogSmoother) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	avg := as.data.Average()
	lastErr := as.lastError.Load()
	if lastErr == nil {
		return avg, nil
	}
	//nolint:forcetypeassert
	lastErrVal := lastErr.(errValue)
	if lastErrVal.present {
		return avg, lastErrVal.err
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
	as.data = utils.NewRollingAverage(numSamples)
	nanosBetween := 1e9 / as.SamplesPerSecond

	as.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		for {
			start := time.Now()
			reading, err := as.Raw.Read(ctx, nil)
			as.lastError.Store(errValue{err != nil, err})
			if err != nil {
				if errors.Is(err, errStopReading) {
					break
				}
				as.logger.Infow("error reading analog", "error", err)
				continue
			}

			as.data.Add(reading)

			end := time.Now()

			toSleep := int64(nanosBetween) - (end.UnixNano() - start.UnixNano())
			if !goutils.SelectContextOrWait(ctx, time.Duration(toSleep)) {
				return
			}
		}
	}, as.activeBackgroundWorkers.Done)
}
