package ultrasonic

import (
	"context"
	"testing"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/sensor/ultrasonic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
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
func TestNewCamera(t *testing.T) {
	fakecfg := &ultrasonic.Config{TriggerPin: triggerPin, EchoInterrupt: echoInterrupt, Board: board1}
	name := resource.Name{API: camera.API}
	ctx := context.Background()
	deps := setupDependencies(t)
	_, err := newCamera(ctx, deps, name, fakecfg, nil)
	test.That(t, err, test.ShouldBeNil)
}
