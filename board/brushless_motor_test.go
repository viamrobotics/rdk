package board

import (
	"context"
	"testing"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"github.com/stretchr/testify/assert"
)

func TestBrushlessMotor(t *testing.T) {
	ctx := context.Background()
	b := &testGPIOBoard{}
	logger := golog.NewTestLogger(t)

	m, err := NewGPIOMotor(b, MotorConfig{Pins: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "pwm": "5"}, TicksPerRotation: 200}, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		test.That(t, utils.TryClose(m), test.ShouldBeNil)
	}()

	assert.Nil(t, m.Off(ctx))
	assert.Equal(t, false, b.gpio["1"])
	assert.Equal(t, false, b.gpio["2"])
	assert.Equal(t, false, b.gpio["3"])
	assert.Equal(t, false, b.gpio["4"])
	on, err := m.IsOn(ctx)
	assert.Nil(t, err)
	assert.False(t, on)

	supported, err := m.PositionSupported(ctx)
	assert.Nil(t, err)
	assert.True(t, supported)

	waitTarget := func(target float64) {
		steps, err := m.Position(ctx)
		assert.Nil(t, err)
		var attempts int
		maxAttempts := 5
		for steps != target && attempts < maxAttempts {
			time.Sleep(time.Second)
			attempts++
			steps, err = m.Position(ctx)
			assert.Nil(t, err)
		}
		assert.Equal(t, target, steps)
	}

	assert.Nil(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 200.0, 2.0))
	waitTarget(2)

	assert.Nil(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 200.0, 4.0))
	waitTarget(-2)
}
