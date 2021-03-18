// +build pi

package board

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPiPigpio(t *testing.T) {

	cfg := Config{
		Analogs: []AnalogConfig{{Name: "blue", Pin: "0"}},
		Servos:  []ServoConfig{{Name: "s1", Pin: "16"}},
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
	assert.InDelta(t, 0, v, 100)

	// try to set high
	err = p.GPIOSet(26, true)
	if err != nil {
		t.Fatal(err)
	}

	v, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	assert.InDelta(t, 1023, v, 100)

	// back to low
	err = p.GPIOSet(26, false)
	if err != nil {
		t.Fatal(err)
	}

	v, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	assert.InDelta(t, 0, v, 100)

	// servo
	s := p.Servo("s1")
	if s == nil {
		t.Fatal("no servo s1")
	}

	err = s.Move(90)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, byte(90), s.Current())
	time.Sleep(200 * time.Millisecond)

	err = s.Move(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, byte(0), s.Current())
	time.Sleep(200 * time.Millisecond)

	err = s.Move(180)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, byte(180), s.Current())
	time.Sleep(200 * time.Millisecond)
}
