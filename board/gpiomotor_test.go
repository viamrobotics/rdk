package board

import (
	"context"
	"testing"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestMotor1(t *testing.T) {
	ctx := context.Background()
	b := &FakeBoard{}
	logger := golog.NewTestLogger(t)

	m, err := NewGPIOMotor(b, MotorConfig{Pins: map[string]string{"a": "1", "b": "2", "pwm": "3"}, PWMFreq: 4000}, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldEqual, false)
	test.That(t, b.gpio["2"], test.ShouldEqual, false)
	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, .43), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldEqual, true)
	test.That(t, b.gpio["2"], test.ShouldEqual, false)
	test.That(t, b.pwm["3"], test.ShouldEqual, byte(109))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldEqual, false)
	test.That(t, b.gpio["2"], test.ShouldEqual, true)
	test.That(t, b.pwm["3"], test.ShouldEqual, byte(112))
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeTrue)

	test.That(t, m.Power(ctx, .45), test.ShouldBeNil)
	test.That(t, b.pwm["3"], test.ShouldEqual, byte(114))

	test.That(t, b.pwmFreq["3"], test.ShouldEqual, 4000)
	test.That(t, b.PWMSetFreq(ctx, "3", 8000), test.ShouldBeNil)
	test.That(t, b.pwmFreq["3"], test.ShouldEqual, 8000)

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldEqual, false)
	test.That(t, b.gpio["2"], test.ShouldEqual, false)
	on, err = m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, .44), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldEqual, false)
	test.That(t, b.gpio["2"], test.ShouldEqual, true)
	test.That(t, m.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, .47), test.ShouldBeNil)
	test.That(t, b.gpio["1"], test.ShouldBeFalse)
	test.That(t, b.gpio["2"], test.ShouldBeFalse)

	pos, err := m.Position(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0.0)
	supported, err := m.PositionSupported(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, supported, test.ShouldBeFalse)
}
