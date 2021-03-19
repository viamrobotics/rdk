// +build pi

package board

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.viam.com/robotcore/board"
)

func TestPiPigpio(t *testing.T) {

	cfg := board.Config{
		Analogs: []board.AnalogConfig{{Name: "blue", Pin: "0"}},
		Servos: []board.ServoConfig{
			{Name: "s1", Pin: "16"},
			{Name: "s2", Pin: "29"},
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "35"},
			{Name: "i2", Pin: "31", Type: "servo"},
			{Name: "hall-a", Pin: "38"},
			{Name: "hall-b", Pin: "40"},
		},
		Motors: []board.MotorConfig{
			{
				Name:             "m",
				Pins:             map[string]string{"a": "11", "b": "13", "pwm": "15"},
				Encoder:          "hall-a",
				EncoderB:         "hall-b",
				TicksPerRotation: 100,
			},
		},
	}

	pp, err := NewPigpio(cfg)
	if err != nil {
		t.Fatal(err)
	}

	p := pp.(*piPigpio)

	assert.Equal(t, cfg, p.GetConfig())

	defer func() {
		err := p.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	t.Run("analog test", func(t *testing.T) {
		reader := p.AnalogReader("blue")
		if reader == nil {
			t.Fatalf("no blue?")
		}

		// try to set low
		err = p.GPIOSetBcom(26, false)
		if err != nil {
			t.Fatal(err)
		}

		v, err := reader.Read()
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 0, v, 150)

		// try to set high
		err = p.GPIOSetBcom(26, true)
		if err != nil {
			t.Fatal(err)
		}

		v, err = reader.Read()
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 1023, v, 150)

		// back to low
		err = p.GPIOSetBcom(26, false)
		if err != nil {
			t.Fatal(err)
		}

		v, err = reader.Read()
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 0, v, 150)
	})

	t.Run("physical servo test", func(t *testing.T) {
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
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.GPIOSetBcom(13, false)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(5 * time.Millisecond)

		before := p.DigitalInterrupt("i1").Value()

		err = p.GPIOSetBcom(13, true)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(5 * time.Millisecond)

		after := p.DigitalInterrupt("i1").Value()
		assert.Equal(t, int64(1), after-before)
	})

	t.Run("servo in/out", func(t *testing.T) {
		s := p.Servo("s2")
		if s == nil {
			t.Fatal("no s2")
		}

		err := s.Move(90)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(300 * time.Millisecond)

		assert.InDelta(t, int64(1500), p.DigitalInterrupt("i2").Value(), 500) // this is a tad noisy
	})

	t.Run("motor", func(t *testing.T) {
		m := p.Motor("m")
		err := m.GoFor(board.DirForward, 250, 5)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, m.IsOn())

		loops := 0
		for m.IsOn() {
			time.Sleep(100 * time.Millisecond)
			loops++
			if loops > 100 {
				t.Fatalf("motor didn't move enough, a: %v b: %v",
					p.DigitalInterrupt("hall-a").Value(),
					p.DigitalInterrupt("hall-b").Value(),
				)
			}
		}

	})
}
