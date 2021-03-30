package board

import (
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/stretchr/testify/assert"
)

type testGPIOBoard struct {
	gpio map[string]bool
	pwm  map[string]byte
}

func (b *testGPIOBoard) GPIOSet(pin string, high bool) error {
	if b.gpio == nil {
		b.gpio = map[string]bool{}
	}
	b.gpio[pin] = high
	return nil
}

func (b *testGPIOBoard) PWMSet(pin string, dutyCycle byte) error {
	if b.pwm == nil {
		b.pwm = map[string]byte{}
	}
	b.pwm[pin] = dutyCycle
	return nil
}

func TestMotor1(t *testing.T) {
	b := &testGPIOBoard{}

	m, err := NewGPIOMotor(b, map[string]string{"a": "1", "b": "2", "pwm": "3"})
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, m.Off())
	assert.Equal(t, false, b.gpio["1"])
	assert.Equal(t, false, b.gpio["2"])
	assert.False(t, m.IsOn())

	assert.Nil(t, m.Go(pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 111))
	assert.Equal(t, true, b.gpio["1"])
	assert.Equal(t, false, b.gpio["2"])
	assert.Equal(t, byte(111), b.pwm["3"])
	assert.True(t, m.IsOn())

	assert.Nil(t, m.Go(pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 112))
	assert.Equal(t, false, b.gpio["1"])
	assert.Equal(t, true, b.gpio["2"])
	assert.Equal(t, byte(112), b.pwm["3"])
	assert.True(t, m.IsOn())

	assert.Nil(t, m.Force(113))
	assert.Equal(t, byte(113), b.pwm["3"])

	assert.Nil(t, m.Off())
	assert.Equal(t, false, b.gpio["1"])
	assert.Equal(t, false, b.gpio["2"])
	assert.False(t, m.IsOn())

	assert.Nil(t, m.Go(pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 112))
	assert.Equal(t, false, b.gpio["1"])
	assert.Equal(t, true, b.gpio["2"])
	assert.Nil(t, m.Go(pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED, 121))
	assert.False(t, b.gpio["1"])
	assert.False(t, b.gpio["2"])

	assert.Equal(t, int64(0), m.Position())
	assert.False(t, m.PositionSupported())
}
