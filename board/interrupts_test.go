package board

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "i1",
		Formula: "(+ 1 raw)",
	}

	i, err := CreateDigitalInterrupt(config)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "i1", i.Config().Name)

	assert.Equal(t, int64(1), i.Value())
	i.Tick(true, nowNanosTest())
	assert.Equal(t, int64(2), i.Value())
	i.Tick(false, nowNanosTest())
	assert.Equal(t, int64(2), i.Value())

	c := make(chan bool)
	i.AddCallback(c)

	go func() { i.Tick(true, nowNanosTest()) }()
	v := <-c
	assert.Equal(t, true, v)

	go func() { i.Tick(true, nowNanosTest()) }()
	v = <-c
	assert.Equal(t, true, v)

}

func TestServoInterrupt(t *testing.T) {
	config := DigitalInterruptConfig{
		Name: "s1",
		Type: "servo",
	}

	s, err := CreateDigitalInterrupt(config)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "s1", s.Config().Name)

	now := uint64(0)
	for i := 0; i < 20; i++ {
		s.Tick(true, now)
		now += 1500 * 1000 // this is what we measure
		s.Tick(false, now)
		now += 1000 * 1000 * 1000 // this is between measuremenats
	}

	assert.Equal(t, int64(1500), s.Value())
}

func TestServoInterruptWithPP(t *testing.T) {
	config := DigitalInterruptConfig{
		Name:    "s1",
		Type:    "servo",
		Formula: "(+ 1 raw)",
	}

	s, err := CreateDigitalInterrupt(config)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "s1", s.Config().Name)

	now := uint64(0)
	for i := 0; i < 20; i++ {
		s.Tick(true, now)
		now += 1500 * 1000 // this is what we measure
		s.Tick(false, now)
		now += 1000 * 1000 * 1000 // this is between measuremenats
	}

	assert.Equal(t, int64(1501), s.Value())
}
