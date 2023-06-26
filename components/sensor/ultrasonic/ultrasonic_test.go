package ultrasonic

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSensorName = "ultrasonic1"
	triggerPin     = "some-pin"
	echoInterrupt  = "some-echo-interrupt"
	board1         = "some-board"
)

func setupDependencies(t *testing.T) resource.Dependencies {
	t.Helper()

	deps := make(resource.Dependencies)

	actualBoard := inject.NewBoard(board1)
	actualBoard.DigitalInterruptNamesFunc = func() []string {
		return []string{echoInterrupt}
	}
	injectDigi := &inject.DigitalInterrupt{}
	actualBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return injectDigi, true
	}
	pin := &inject.GPIOPin{}
	pin.SetFunc = func(ctx context.Context, high bool, extra map[string]interface{}) error {
		return nil
	}
	actualBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		return pin, nil
	}
	deps[board.Named(board1)] = actualBoard

	return deps
}

func TestValidate(t *testing.T) {
	fakecfg := &Config{}
	_, err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"board\" is required")

	fakecfg.Board = board1
	_, err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"trigger pin\" is required")

	fakecfg.TriggerPin = triggerPin
	_, err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"echo interrupt pin\" is required")

	fakecfg.EchoInterrupt = echoInterrupt
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSensor(t *testing.T) {
	fakecfg := &Config{TriggerPin: triggerPin, EchoInterrupt: echoInterrupt, Board: board1}
	ctx := context.Background()
	deps := setupDependencies(t)

	_, err := NewSensor(ctx, deps, sensor.Named(testSensorName), fakecfg)
	test.That(t, err, test.ShouldBeNil)
}
