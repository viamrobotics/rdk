package board

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasicDigitalInterrupt1(t *testing.T) {
	config := DigitalInterruptConfig{
		Formula: "(+ 1 raw)",
	}

	i, err := createDigitalInterrupt(config)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int64(1), i.Value())
	i.Tick(true)
	assert.Equal(t, int64(2), i.Value())
	i.Tick(false)
	assert.Equal(t, int64(2), i.Value())

	c := make(chan bool)
	i.AddCallback(c)

	go func() { i.Tick(true) }()
	v := <-c
	assert.Equal(t, true, v)

	go func() { i.Tick(true) }()
	v = <-c
	assert.Equal(t, true, v)

}
