package arduino

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
)

// Test the A/B/PWM style IO
func TestArduinoMotorABPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := board.Config{
		Motors: []board.MotorConfig{
			{
				Name: "m1",
				Pins: map[string]string{
					"pwm": "5",
					"a":   "6",
					"b":   "7",
					"en":  "8",
				},
				Encoder:          "3",
				EncoderB:         "2",
				TicksPerRotation: 2000,
			},
		},
	}
	b, err := newArduino(ctx, cfg, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {

		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, ok := b.MotorByName(cfg.Motors[0].Name)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, m, test.ShouldNotBeNil)

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

}

// Test the DIR/PWM style IO
func TestArduinoMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := board.Config{
		Motors: []board.MotorConfig{
			{
				Name: "m1",
				Pins: map[string]string{
					"pwm": "5",
					"dir": "6",
					"en":  "7",
				},
				Encoder:          "3",
				EncoderB:         "2",
				TicksPerRotation: 2000,
			},
		},
	}
	b, err := newArduino(ctx, cfg, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {

		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, ok := b.MotorByName(cfg.Motors[0].Name)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, m, test.ShouldNotBeNil)

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

}

// Test the A/B only style IO
func TestArduinoMotorAB(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := board.Config{
		Motors: []board.MotorConfig{
			{
				Name: "m1",
				Pins: map[string]string{
					"a":  "5",
					"b":  "6",
					"en": "7",
				},
				Encoder:          "3",
				EncoderB:         "2",
				TicksPerRotation: 2000,
			},
		},
	}
	b, err := newArduino(ctx, cfg, logger)
	if err != nil && strings.HasPrefix(err.Error(), "found ") {

		t.Skip()
		return
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, b, test.ShouldNotBeNil)
	defer b.Close()

	m, ok := b.MotorByName(cfg.Motors[0].Name)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, m, test.ShouldNotBeNil)

	startPos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 20, 1.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeFalse)

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos-startPos, test.ShouldBeGreaterThan, 1)
	})

	err = m.Off(ctx)
	test.That(t, err, test.ShouldBeNil)

	utils.SelectContextOrWait(ctx, 500*time.Millisecond)

	err = m.Zero(ctx, 2.0)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	err = m.GoTo(ctx, 50, 0.5)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(t testing.TB) {
		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldBeLessThan, 1)
	})

}
