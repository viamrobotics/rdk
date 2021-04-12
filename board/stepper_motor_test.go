package board

import (
	"context"
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/stretchr/testify/assert"
)



func TestStepperMotor(t *testing.T) {
	ctx := context.Background()
	b := &testGPIOBoard{}

	m, err := NewGPIOMotor(b, MotorConfig{Pins: map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "pwm": "5"},TicksPerRotation: 200})
	if err != nil {
		t.Fatal(err)
	}

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
	
	assert.Nil(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 50.0, 40.0))
	steps, err := m.Position(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 160.0, steps)
	
	assert.Nil(t, m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 50.0, 20.0))
	steps, err = m.Position(ctx)
	assert.Nil(t, err)
	assert.Equal(t, 80.0, steps)

}
