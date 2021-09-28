package board_test

import (
	"context"
	"testing"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robots/fake"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestGPIOStepperMotor(t *testing.T) {
	ctx := context.Background()
	b := &fake.Board{}
	logger := golog.NewTestLogger(t)

	m, err := board.NewGPIOMotor(b, motor.Config{Pins: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "pwm": "5"}, TicksPerRotation: 200}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	}()

	test.That(t, m.Off(ctx), test.ShouldBeNil)
	test.That(t, b.GPIO["1"], test.ShouldEqual, false)
	test.That(t, b.GPIO["2"], test.ShouldEqual, false)
	test.That(t, b.GPIO["3"], test.ShouldEqual, false)
	test.That(t, b.GPIO["4"], test.ShouldEqual, false)
	on, err := m.IsOn(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, on, test.ShouldBeFalse)

	supported, err := m.PositionSupported(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, supported, test.ShouldBeTrue)

	waitTarget := func(target float64) {
		steps, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		var attempts int
		maxAttempts := 5
		for steps != target && attempts < maxAttempts {
			time.Sleep(time.Second)
			attempts++
			steps, err = m.Position(ctx)
			test.That(t, err, test.ShouldBeNil)
		}
		test.That(t, steps, test.ShouldEqual, target)
	}

	test.That(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 200.0, 2.0), test.ShouldBeNil)
	waitTarget(2)

	test.That(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 200.0, 4.0), test.ShouldBeNil)
	waitTarget(-2)
}
