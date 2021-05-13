// +build pi

package pi

import (
	"context"
	"testing"
	"time"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
	"go.viam.com/test"
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
	test.That(t, err, test.ShouldBeNil)

	p := pp.(*piPigpio)

	defer func() {
		err := p.Close()
		test.That(t, err, test.ShouldBeNil)
	}()

	cfgGet, err := p.Config(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfgGet, test.ShouldResemble, cfg)
	t.Run("analog test", func(t *testing.T) {
		reader := p.AnalogReader("blue")
		if reader == nil {
			t.Skip("no blue? analog")
			return
		}

		// try to set low
		err = p.GPIOSetBcom(26, false)
		test.That(t, err, test.ShouldBeNil)

		v, err := reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)

		// try to set high
		err = p.GPIOSetBcom(26, true)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 1023, 150)

		// back to low
		err = p.GPIOSetBcom(26, false)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.GPIOSetBcom(18, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		before := p.DigitalInterrupt("i1").Value()

		err = p.GPIOSetBcom(18, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after := p.DigitalInterrupt("i1").Value()
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	t.Run("servo in/out", func(t *testing.T) {
		s := p.Servo("servo")
		test.That(t, s, test.ShouldNotBeNil)

		err := s.Move(ctx, 90)
		test.That(t, err, test.ShouldBeNil)

		v, err := s.Current(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 90)

		time.Sleep(300 * time.Millisecond)

		test.That(t, p.DigitalInterrupt("servo-i").Value(), test.ShouldAlmostEqual, int64(1500), 500) // this is a tad noisy
	})

	t.Run("motor forward", func(t *testing.T) {
		m := p.Motor("m")

		pos, err := m.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldAlmostEqual, .0, 01)

		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err = m.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 15, 2)
		test.That(t, err, test.ShouldBeNil)
		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)

			loops++
			if loops > 100 {
				pos, err = m.Position(ctx)
				test.That(t, err, test.ShouldBeNil)
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
		test.That(t, err, test.ShouldBeNil)

		on, err := m.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := m.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
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
