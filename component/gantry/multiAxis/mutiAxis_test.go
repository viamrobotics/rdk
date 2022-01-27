package multiAxis

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/inject"
)

func createFakeMotor() *inject.Motor {
	fakemotor := &inject.Motor{}

	fakemotor.PositionSupportedFunc = func(ctx context.Context) (bool, error) {
		return true, nil
	}

	fakemotor.PositionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}

	fakemotor.GoForFunc = func(ctx context.Context, rpm float64, revolutions float64) error {
		return nil
	}

	fakemotor.StopFunc = func(ctx context.Context) error {
		return nil
	}

	fakemotor.GoFunc = func(ctx context.Context, powerPct float64) error {
		return nil
	}

	return fakemotor
}

func createFakeBoard() *inject.Board {
	fakeboard := &inject.Board{}

	fakeboard.GPIOGetFunc = func(ctx context.Context, pin string) (bool, error) {
		return true, nil
	}
	return fakeboard
}

func createFakeOneAxis() *inject.Gantry {
	fakeoneaxis := &inject.Gantry{}
	return fakeoneaxis
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

func TestInit(t *testing.T) {
	ctx := context.Background()
	fakemotor := createFakeMotor()
	_, err := fakemotor.PositionSupportedFunc(ctx)
	test.That(t, err, test.ShouldBeNil)
}
