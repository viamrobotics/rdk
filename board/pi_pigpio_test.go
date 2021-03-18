// +build pi

package board

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPiPigpio(t *testing.T) {

	cfg := Config{
		Analogs: []AnalogConfig{{Name: "blue", Pin: "0"}},
	}

	p, err := NewPigpio(cfg)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := p.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	reader := p.AnalogReader("blue")
	if reader == nil {
		t.Fatalf("no blue?")
	}

	// try to set low
	err = p.GPIOSet(26, false)
	if err != nil {
		t.Fatal(err)
	}

	v, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, v)

	// try to set high
	err = p.GPIOSet(26, true)
	if err != nil {
		t.Fatal(err)
	}

	v, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1023, v)

}
