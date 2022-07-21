package ultrasonic

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/registry"
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
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"board\" is required")

	fakecfg.Board = board1
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"trigger pin\" is required")

	fakecfg.TriggerPin = triggerPin
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"path\": \"echo interrupt pin\" is required")

	fakecfg.EchoInterrupt = echoInterrupt
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSensor(t *testing.T) {
	fakecfg := &AttrConfig{TriggerPin: triggerPin, EchoInterrupt: echoInterrupt, Board: board1}
	ctx := context.Background()
	deps := setupDependencies(t)

	_, err := newSensor(ctx, deps, testSensorName, fakecfg)

	test.That(t, err.Error(), test.ShouldContainSubstring, "ultrasonic: cannot find board \"some-board\"")
}

// Mock DigitalInterrupt.
type mockDigitalInterrupt struct{}

// mock board.
type mock struct {
	board.LocalBoard
	Name     string
	digitals []string
	digital  *mockDigitalInterrupt
}

func newBoard(name string) *mock {
	return &mock{
		Name:     name,
		digitals: []string{echoInterrupt},
		digital:  &mockDigitalInterrupt{},
	}
}

func (m *mock) DigitalInterruptByName(name string) (*mockDigitalInterrupt, bool) {
	if len(m.digitals) == 0 {
		return nil, false
	}
	return m.digital, true
}
