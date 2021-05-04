package board

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/utils"
)

var (
	ErrStopReading = errors.New("stop reading")
)

func AnalogSmootherWrap(r AnalogReader, c AnalogConfig, logger golog.Logger) AnalogReader {
	if c.AverageOverMillis <= 0 {
		return r
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	as := &AnalogSmoother{
		Raw:               r,
		AverageOverMillis: c.AverageOverMillis,
		SamplesPerSecond:  c.SamplesPerSecond,
		logger:            logger,
		cancel:            cancel,
	}
	as.Start(cancelCtx)
	return as
}

type AnalogSmoother struct {
	Raw                     AnalogReader
	AverageOverMillis       int
	SamplesPerSecond        int
	data                    *utils.RollingAverage
	lastError               atomic.Value // errValue
	logger                  golog.Logger
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
}

type errValue struct {
	present bool
	err     error
}

func (as *AnalogSmoother) Close() error {
	as.cancel()
	as.activeBackgroundWorkers.Wait()
	return nil
}

func (as *AnalogSmoother) Read(ctx context.Context) (int, error) {
	avg := as.data.Average()
	lastErr := as.lastError.Load()
	if lastErr == nil {
		return avg, nil
	}
	lastErrVal := lastErr.(errValue)
	if lastErrVal.present {
		return avg, lastErrVal.err
	}
	return avg, nil
}

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
	utils.ManagedGo(func() {
		for {
			start := time.Now()
			reading, err := as.Raw.Read(ctx)
			as.lastError.Store(errValue{err != nil, err})
			if err != nil {
				if err == ErrStopReading {
					break
				}
				as.logger.Info("error reading analog: %s", err)
				continue
			}

			//as.logger.Debugf("reading: %d", reading)
			as.data.Add(reading)

			end := time.Now()

			toSleep := int64(nanosBetween) - (end.UnixNano() - start.UnixNano())
			if !utils.SelectContextOrWait(ctx, time.Duration(toSleep)) {
				return
			}
		}
	}, as.activeBackgroundWorkers.Done)
}
