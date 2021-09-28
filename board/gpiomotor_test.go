package board_test

import (
	"context"
	"testing"

	"go.viam.com/core/board"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

// Test the A/B/PWM style IO
func TestMotorABPWM(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	m, err := board.NewGPIOMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .43), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, true)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(109))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, true)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(112))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Power(ctx, .45), test.ShouldBeNil)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(114))

	test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
	test.That(t, b.PWMSetFreq(ctx, "3", 8000), test.ShouldBeNil)
	test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(0))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, true)
	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, .47), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.GPIO["3"], test.ShouldEqual, false)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0.0)
	supported, err := m.PositionSupported(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, supported, test.ShouldBeFalse)
}

// Test the DIR/PWM style IO
func TestMotorDirPWM(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	m, err := board.NewGPIOMotor(b, motor.Config{Pins: map[string]string{"dir": "1", "en": "2", "pwm": "3"}, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, true)
	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .43), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, true)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(109))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(112))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Power(ctx, .45), test.ShouldBeNil)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(114))

	test.That(t, b.PWMFreq["3"], test.ShouldEqual, 4000)
	test.That(t, b.PWMSetFreq(ctx, "3", 8000), test.ShouldBeNil)
	test.That(t, b.PWMFreq["3"], test.ShouldEqual, 8000)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, true)
	test.That(t, b.PWM["3"], test.ShouldEqual, byte(0))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

}

// Test the A/B only style IO
func TestMotorAB(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	m, err := board.NewGPIOMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "en": "3"}, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.GPIO["3"], test.ShouldEqual, true)
	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .43), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, true)
	test.That(t, b.PWM["2"], test.ShouldEqual, byte(145))
	test.That(t, b.GPIO["3"], test.ShouldEqual, false)
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.GPIO["2"], test.ShouldEqual, true)
	test.That(t, b.PWM["1"], test.ShouldEqual, byte(142))
	test.That(t, b.GPIO["3"], test.ShouldEqual, false)
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Power(ctx, .45), test.ShouldBeNil)
	test.That(t, b.PWM["1"], test.ShouldEqual, byte(140))

	test.That(t, b.PWMFreq["1"], test.ShouldEqual, 4000)
	test.That(t, b.PWMSetFreq(ctx, "1", 8000), test.ShouldBeNil)
	test.That(t, b.PWMFreq["1"], test.ShouldEqual, 8000)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.PWM["1"], test.ShouldEqual, 0)
	test.That(t, b.PWM["2"], test.ShouldEqual, 0)
	test.That(t, b.GPIO["3"], test.ShouldEqual, true)

	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

}
