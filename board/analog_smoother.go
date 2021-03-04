package board

import (
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/utils"
)

type AnalogSmoother struct {
	Raw               AnalogReader
	AverageOverMillis int
	SamplesPerSecond  int
	data              *utils.RollingAverage
}

func (as *AnalogSmoother) Read() (int, error) {
	return as.data.Average(), nil
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
			if err != nil {
				golog.Global.Info("error reading analog: %s", err)
				continue
			}

			//golog.Global.Debugf("reading: %d", reading)
			as.data.Add(reading)

			end := time.Now()

			toSleep := int64(nanosBetween) - (end.UnixNano() - start.UnixNano())
			time.Sleep(time.Duration(toSleep))
		}
	}()
}
