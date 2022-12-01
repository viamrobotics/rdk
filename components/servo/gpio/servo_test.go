// Package gpio implements a pin based servo
package gpio

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

func Ptr[T any](v T) *T {
	return &v
}

func TestValidate(t *testing.T) {
	cfg := servoConfig{
		Pin:        "a",
		Board:      "b",
		MinDeg:     Ptr(1.5),
		MaxDeg:     Ptr(90.0),
		StartPos:   Ptr(3.5),
		MinWidthUS: Ptr(uint(501)),
		MaxWidthUS: Ptr(uint(2499)),
	}

	deps, err := cfg.Validate("test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldContain, "b")
	test.That(t, len(deps), test.ShouldEqual, 1)

	cfg.MinDeg = Ptr(-1.5)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": min_angle_deg cannot be lower than 0")
	cfg.MinDeg = Ptr(1.5)

	cfg.MaxDeg = Ptr(90.0)

	cfg.MinWidthUS = Ptr(uint(450))
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": min_width_us cannot be lower than 500")
	cfg.MinWidthUS = Ptr(uint(501))

	cfg.MaxWidthUS = Ptr(uint(2520))
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": max_width_us cannot be higher than 2500")
	cfg.MaxWidthUS = Ptr(uint(2499))

	cfg.StartPos = Ptr(91.0)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_degs should be between 1.5 and 90.0")

	cfg.StartPos = Ptr(1.0)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_degs should be between 1.5 and 90.0")

	cfg.StartPos = Ptr(199.0)
	cfg.MaxDeg = nil
	cfg.MinDeg = nil
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_degs should be between 0.0 and 180.0")

	cfg.StartPos = Ptr(0.0)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldBeNil)

	cfg.Board = ""
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": \"board\" is required")
	cfg.Board = "b"

	cfg.Pin = ""
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": \"pin\" is required")
}

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	gpio := make(map[string]board.GPIOPin)
	gpio["0"] = &mockGPIO{scale: 255, frequency: 50, innerTick: 0}
	gpio["1"] = &mockGPIO{scale: 4095, frequency: 50, innerTick: 0}
	deps[board.Named("mock")] = &mockBoard{gpio: gpio}
	return deps
}

type mockGPIO struct {
	board.GPIOPin
	scale     int
	frequency int
	innerTick int
}

func (g *mockGPIO) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return uint(g.frequency), nil
}

func (g *mockGPIO) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	g.frequency = int(freqHz)
	return nil
}

func (g *mockGPIO) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	pct := float64(g.innerTick) / float64(g.scale)
	return pct, nil
}

func (g *mockGPIO) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	g.innerTick = rdkutils.ScaleByPct(g.scale, dutyCyclePct)
	return nil
}

type mockBoard struct {
	board.LocalBoard
	gpio map[string]board.GPIOPin
}

func (b *mockBoard) GPIOPinByName(name string) (board.GPIOPin, error) {
	return b.gpio[name], nil
}

func TestServoMove(t *testing.T) {
	logger := golog.NewTestLogger(t)
	deps := setupDependencies(t)

	ctx := context.Background()

	attrs := servoConfig{
		Pin:      "0",
		Board:    "mock",
		StartPos: Ptr(0.0),
	}

	cfg := config.Component{
		ConvertedAttributes: &attrs,
	}
	servo, err := newGPIOServo(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, servo, test.ShouldNotBeNil)
	realServo, ok := servo.(*servoGPIO)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, realServo.pwmRes, test.ShouldEqual, 255)
	pos, err := realServo.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	err = realServo.Move(ctx, 63, nil)
	test.That(t, err, test.ShouldBeNil)
	pos, err = realServo.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 61)

	attrs = servoConfig{
		Pin:      "1",
		Board:    "mock",
		StartPos: Ptr(0.0),
	}

	cfg = config.Component{
		ConvertedAttributes: &attrs,
	}
	servo, err = newGPIOServo(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, servo, test.ShouldNotBeNil)
	realServo, ok = servo.(*servoGPIO)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, realServo.pwmRes, test.ShouldEqual, 4095)
	pos, err = realServo.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	err = realServo.Move(ctx, 63, nil)
	test.That(t, err, test.ShouldBeNil)
	pos, err = realServo.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 63)
}
