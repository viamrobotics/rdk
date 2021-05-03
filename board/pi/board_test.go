// +build pi

package pi

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"

	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := board.Config{
		//Analogs: []board.AnalogConfig{{Name: "blue", Pin: "0"}},
		Servos: []board.ServoConfig{
			{Name: "servo", Pin: "18"}, // bcom-24
		},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"},                     // plug physical 12(18) into this (17)
			{Name: "servo-i", Pin: "22", Type: "servo"}, // bcom-25
			{Name: "hall-a", Pin: "33"},                 // bcom 13
			{Name: "hall-b", Pin: "37"},                 // bcom 26
		},
		Motors: []board.MotorConfig{
			{
				Name: "m",
				Pins: map[string]string{
					"a":   "13", // bcom 27
					"b":   "40", // bcom 21
					"pwm": "7",  // bcom 4
				},
				Encoder:          "hall-a",
				EncoderB:         "hall-b",
				TicksPerRotation: 200,
			},
		},
	}

	pp, err := NewPigpio(ctx, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	p := pp.(*piPigpio)

	defer func() {
		err := p.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	cfgGet, err := p.GetConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, cfg, cfgGet)
	t.Run("analog test", func(t *testing.T) {
		reader := p.AnalogReader("blue")
		if reader == nil {
			t.Skip("no blue? analog")
			return
		}

		// try to set low
		err = p.GPIOSetBcom(26, false)
		if err != nil {
			t.Fatal(err)
		}

		v, err := reader.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 0, v, 150)

		// try to set high
		err = p.GPIOSetBcom(26, true)
		if err != nil {
			t.Fatal(err)
		}

		v, err = reader.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 1023, v, 150)

		// back to low
		err = p.GPIOSetBcom(26, false)
		if err != nil {
			t.Fatal(err)
		}

		v, err = reader.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 0, v, 150)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.GPIOSetBcom(18, false)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(5 * time.Millisecond)

		before := p.DigitalInterrupt("i1").Value()

		err = p.GPIOSetBcom(18, true)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(5 * time.Millisecond)

		after := p.DigitalInterrupt("i1").Value()
		assert.Equal(t, int64(1), after-before)
	})

	t.Run("servo in/out", func(t *testing.T) {
		s := p.Servo("servo")
		if s == nil {
			t.Fatal("no servo")
		}

		err := s.Move(ctx, 90)
		if err != nil {
			t.Fatal(err)
		}

		v, err := s.Current(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, 90, int(v))

		time.Sleep(300 * time.Millisecond)

		assert.InDelta(t, int64(1500), p.DigitalInterrupt("servo-i").Value(), 500) // this is a tad noisy
	})

	t.Run("motor forward", func(t *testing.T) {
		m := p.Motor("m")

		pos, err := m.Position(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.InDelta(t, 0, pos, .01)

		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 15, 2)
		if err != nil {
			t.Fatal(err)
		}
		on, err := m.IsOn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, on)

		loops := 0
		for {
			on, err := m.IsOn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)

			loops++
			if loops > 100 {
				pos, err = m.Position(ctx)
				if err != nil {
					t.Fatal(err)
				}
				t.Fatalf("motor didn't move enough, a: %v b: %v pos: %v",
					p.DigitalInterrupt("hall-a").Value(),
					p.DigitalInterrupt("hall-b").Value(),
					pos,
				)
			}
		}

	})

	t.Run("motor backward", func(t *testing.T) {
		m := p.Motor("m")
		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err := m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 15, 2)
		if err != nil {
			t.Fatal(err)
		}

		on, err := m.IsOn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, on)

		loops := 0
		for {
			on, err := m.IsOn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !on {
				break
			}

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
