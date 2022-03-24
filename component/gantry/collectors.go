package gantry

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/data"
)

type method int64

const (
	getPosition method = iota
	getLengths
)

func (m method) String() string {
	switch m {
	case getPosition:
		return "GetPosition"
	case getLengths:
		return "GetLengths"
	}
	return "Unknown"
}

func newGetPositionCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, logger golog.Logger) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetPosition(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getPosition.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func newGetLengthsCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, logger golog.Logger) (data.Collector, error) {
	gantry, err := assertGantry(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gantry.GetLengths(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, getLengths.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func assertGantry(resource interface{}) (Gantry, error) {
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return gantry, nil
}
