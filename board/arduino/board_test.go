package arduino

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
)

func TestArduino(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := board.Config{
		Motors: []board.MotorConfig{
			{
				Name: "m1",
				Pins: map[string]string{
					"pwm": "28",
					"a":   "29",
					"b":   "30",
				},
				Encoder:          "3",
				EncoderB:         "2",
				TicksPerRotation: 7500,
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

	m := b.Motor(cfg.Motors[0].Name)
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

}
