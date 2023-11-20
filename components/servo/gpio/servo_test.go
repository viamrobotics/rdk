// Package gpio implements a pin based servo
package gpio

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func ptr[T any](v T) *T {
	return &v
}

func TestValidate(t *testing.T) {
	cfg := servoConfig{
		Pin:        "a",
		Board:      "b",
		MinDeg:     ptr(1.5),
		MaxDeg:     ptr(90.0),
		StartPos:   ptr(3.5),
		MinWidthUs: ptr(uint(501)),
		MaxWidthUs: ptr(uint(2499)),
	}

	deps, err := cfg.Validate("test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldContain, "b")
	test.That(t, len(deps), test.ShouldEqual, 1)

	cfg.MinDeg = ptr(-1.5)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": min_angle_deg cannot be lower than 0")
	cfg.MinDeg = ptr(1.5)

	cfg.MaxDeg = ptr(90.0)

	cfg.MinWidthUs = ptr(uint(450))
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": min_width_us cannot be lower than 500")
	cfg.MinWidthUs = ptr(uint(501))

	cfg.MaxWidthUs = ptr(uint(2520))
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "error validating \"test\": max_width_us cannot be higher than 2500")
	cfg.MaxWidthUs = ptr(uint(2499))

	cfg.StartPos = ptr(91.0)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_deg should be between minimum (1.5) and maximum (90.0) positions")

	cfg.StartPos = ptr(1.0)
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_deg should be between minimum (1.5) and maximum (90.0) positions")

	cfg.StartPos = ptr(199.0)
	cfg.MaxDeg = nil
	cfg.MinDeg = nil
	_, err = cfg.Validate("test")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(),
		test.ShouldContainSubstring,
		"error validating \"test\": starting_position_deg should be between minimum (0.0) and maximum (180.0) positions")

	cfg.StartPos = ptr(0.0)
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

func setupDependencies(t *testing.T) resource.Dependencies {
	t.Helper()

	deps := make(resource.Dependencies)
	board1 := inject.NewBoard("mock")

	innerTick1, innerTick2 := 0, 0
	scale1, scale2 := 255, 4095

	pin0 := &inject.GPIOPin{}
	pin0.PWMFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		pct := float64(innerTick1) / float64(scale1)
		return pct, nil
	}
	pin0.PWMFreqFunc = func(ctx context.Context, extra map[string]interface{}) (uint, error) {
		return 50, nil
	}
	pin0.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
		innerTick1 = utils.ScaleByPct(scale1, dutyCyclePct)
		return nil
	}
	pin0.SetPWMFreqFunc = func(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
		return nil
	}

	pin1 := &inject.GPIOPin{}
	pin1.PWMFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		pct := float64(innerTick2) / float64(scale2)
		return pct, nil
	}
	pin1.PWMFreqFunc = func(ctx context.Context, extra map[string]interface{}) (uint, error) {
		return 50, nil
	}
	pin1.SetPWMFunc = func(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
		innerTick2 = utils.ScaleByPct(scale2, dutyCyclePct)
		return nil
	}
	pin1.SetPWMFreqFunc = func(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
		return nil
	}

	board1.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		switch name {
		case "0":
			return pin0, nil
		case "1":
			return pin1, nil
		default:
			return nil, errors.New("bad pin")
		}
	}
	deps[board.Named("mock")] = board1
	return deps
}

func TestServoMove(t *testing.T) {
	logger := logging.NewTestLogger(t)
	deps := setupDependencies(t)

	ctx := context.Background()

	conf := servoConfig{
		Pin:      "0",
		Board:    "mock",
		StartPos: ptr(0.0),
	}

	cfg := resource.Config{
		ConvertedAttributes: &conf,
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

	conf = servoConfig{
		Pin:      "1",
		Board:    "mock",
		StartPos: ptr(0.0),
	}

	cfg = resource.Config{
		ConvertedAttributes: &conf,
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
