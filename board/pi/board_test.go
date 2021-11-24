//go:build pi
// +build pi

package pi

import (
	"context"
	"testing"
	"time"

	"go.viam.com/core/board"
	"go.viam.com/core/component/servo"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := board.Config{
		//Analogs: []board.AnalogConfig{{Name: "blue", Pin: "0"}},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"},                     // plug physical 12(18) into this (17)
			{Name: "servo-i", Pin: "22", Type: "servo"}, // bcom-25
			{Name: "hall-a", Pin: "33"},                 // bcom 13
			{Name: "hall-b", Pin: "37"},                 // bcom 26
		},
	}

	pp, err := NewPigpio(ctx, &cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	servoCtor := registry.ComponentLookup(servo.Subtype, modelName)
	servo1, err := servoCtor(ctx, nil, config.Component{Name: "servo", Attributes: config.AttributeMap{"pin": "18"}}, logger)
	test.That(t, err, test.ShouldBeNil)

	p := pp.(*piPigpio)

	defer func() {
		err := p.Close()
		test.That(t, err, test.ShouldBeNil)
	}()

	t.Run("analog test", func(t *testing.T) {
		reader, ok := p.AnalogReaderByName("blue")
		test.That(t, ok, test.ShouldBeTrue)
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

		i1, ok := p.DigitalInterruptByName("i1")
		test.That(t, ok, test.ShouldBeTrue)
		before, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)

		err = p.GPIOSetBcom(18, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	t.Run("servo in/out", func(t *testing.T) {
		err := servo1.Move(ctx, 90)
		test.That(t, err, test.ShouldBeNil)

		v, err := servo1.AngularOffset(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 90)

		time.Sleep(300 * time.Millisecond)

		servoI, ok := p.DigitalInterruptByName("servo-i")
		test.That(t, ok, test.ShouldBeTrue)
		val, err := servoI.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldAlmostEqual, int64(1500), 500) // this is a tad noisy
	})

	injectRobot := Robot{}
	injectRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
		return pp, true
	}

	motorCtor := registry.MotorLookup(modelName)
	motor1, err := motorCtor(ctx, &injectRobot, config.Component{Name: "motor1", ConvertedAttributes: &motor.Config{
		Pins: map[string]string{
			"a":   "13", // bcom 27
			"b":   "40", // bcom 21
			"pwm": "7",  // bcom 4
		},
		Encoder:          "hall-a",
		EncoderB:         "hall-b",
		TicksPerRotation: 200,
	},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("motor forward", func(t *testing.T) {
		pos, err := motor1.Position(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldAlmostEqual, .0, 01)

		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err = motor1.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, 15, 2)
		test.That(t, err, test.ShouldBeNil)
		on, err := motor1.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		hallA, ok := p.DigitalInterruptByName("hall-a")
		test.That(t, ok, test.ShouldBeTrue)
		hallB, ok := p.DigitalInterruptByName("hall-b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := motor1.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)

			loops++
			if loops > 100 {
				pos, err = motor1.Position(ctx)
				test.That(t, err, test.ShouldBeNil)
				aVal, err := hallA.Value(context.Background())
				test.That(t, err, test.ShouldBeNil)
				bVal, err := hallB.Value(context.Background())
				test.That(t, err, test.ShouldBeNil)
				t.Fatalf("motor didn't move enough, a: %v b: %v pos: %v",
					aVal,
					bVal,
					pos,
				)
			}
		}

	})

	t.Run("motor backward", func(t *testing.T) {
		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err := motor1.GoFor(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, 15, 2)
		test.That(t, err, test.ShouldBeNil)

		on, err := motor1.IsOn(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		hallA, ok := p.DigitalInterruptByName("hall-a")
		test.That(t, ok, test.ShouldBeTrue)
		hallB, ok := p.DigitalInterruptByName("hall-b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := motor1.IsOn(ctx)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)
			loops++
			aVal, err := hallA.Value(context.Background())
			test.That(t, err, test.ShouldBeNil)
			bVal, err := hallB.Value(context.Background())
			test.That(t, err, test.ShouldBeNil)
			if loops > 100 {
				t.Fatalf("motor didn't move enough, a: %v b: %v",
					aVal,
					bVal,
				)
			}
		}

	})

}

type Robot struct {
	robot.Robot
	BoardByNameFunc func(name string) (board.Board, bool)
}

// BoardByName calls the injected BoardByName or the real version.
func (r *Robot) BoardByName(name string) (board.Board, bool) {
	if r.BoardByNameFunc == nil {
		return r.Robot.BoardByName(name)
	}
	return r.BoardByNameFunc(name)
}
