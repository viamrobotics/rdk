package board

import (
	"errors"
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

	as := &AnalogSmoother{
		Raw:               r,
		AverageOverMillis: c.AverageOverMillis,
		SamplesPerSecond:  c.SamplesPerSecond,
		logger:            logger,
	}
	as.Start()
	return as
}

type AnalogSmoother struct {
	Raw               AnalogReader
	AverageOverMillis int
	SamplesPerSecond  int
	data              *utils.RollingAverage
	lastError         error
	logger            golog.Logger
}

func (as *AnalogSmoother) Read() (int, error) {
	return as.data.Average(), as.lastError
}

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
	as.data = utils.NewRollingAverage(numSamples)
	nanosBetween := 1e9 / as.SamplesPerSecond

	go func() {
		for {
			start := time.Now()
			reading, err := as.Raw.Read()
			as.lastError = err
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
			time.Sleep(time.Duration(toSleep))
		}
	}()
}
