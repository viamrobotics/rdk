package multiaxisgantry

import (
	"context"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/testutils/inject"

	"go.viam.com/test"
)

func createShamMotor() *inject.Motor {
	shamMotor := &inject.Motor{}

	shamMotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	shamMotor.GoTillStopFunc = func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
		return nil
	}
	shamMotor.GoToFunc = func(ctx context.Context, rpm float64, position float64) error {
		return nil
	}

	shamMotor.OffFunc = func(ctx context.Context) error {
		return nil
	}

	shamMotor.GoFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}

	return shamMotor
}

func TestNewMultiAxis(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	shamRobot := &inject.Robot{}

	shamcfg := config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{},
			"limitSwitchPins": []string{},
			"lengthMeters":    []float64{},
			"board":           "",
		},
	}
	_, err := NewMultiAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)

	shamcfg = config.Component{
		Attributes: config.AttributeMap{
			"motorList":       []string{"x", "y", "z"},
			"limitSwitchPins": []string{"1", "2", "3", "4", "5", "6"},
			"lengthMeters":    []float64{1.0, 1.0, 1.0},
			"board":           "board",
		},
	}

	_, err = NewMultiAxis(ctx, shamRobot, shamcfg, logger)
	test.That(t, err, test.ShouldNotBeNil)

}
