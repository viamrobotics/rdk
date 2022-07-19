package ultrasonic

import (
	"context"
	"testing"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/registry"
	"go.viam.com/test"
)

const (
	testSensorName = "ultrasonic1"
	triggerPin     = "some-pin"
	echoInterrupt  = "some-echo-interrupt"
	board1         = "some-board"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)

	actualBoard := newBoard(board1)
	deps[board.Named(board1)] = actualBoard

	return deps
}

func TestValidate(t *testing.T) {
	fakecfg := &AttrConfig{}
	err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find board for ultrasonic sensor")

	fakecfg.Board = board1
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty trigger pin")

	fakecfg.TriggerPin = triggerPin
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty echo interrupt pin")

	fakecfg.EchoInterrupt = echoInterrupt
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSensor(t *testing.T) {
	fakecfg := &AttrConfig{TriggerPin: triggerPin, EchoInterrupt: echoInterrupt, Board: board1}
	ctx := context.Background()
	deps := setupDependencies(t)

	_, err := newSensor(ctx, deps, testSensorName, fakecfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "ultrasonic: cannot find board")
}

// mock board
type mockBoard struct {
	board.LocalBoard
	Name string
}

func newBoard(name string) *mockBoard {
	return &mockBoard{
		Name: name,
	}
}
