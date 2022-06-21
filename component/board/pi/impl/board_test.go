//go:build linux && (arm64 || arm)

package piimpl

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	picommon "go.viam.com/rdk/component/board/pi/common"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"

	// for gpio motor.
	_ "go.viam.com/rdk/component/motor/gpio"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := board.Config{
		// Analogs: []board.AnalogConfig{{Name: "blue", Pin: "0"}},
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"},                     // plug physical 12(18) into this (17)
			{Name: "servo-i", Pin: "22", Type: "servo"}, // bcom-25
			{Name: "hall-a", Pin: "33"},                 // bcom 13
			{Name: "hall-b", Pin: "37"},                 // bcom 26
		},
	}

	pp, err := NewPigpio(ctx, &cfg, logger)
	if errors.Is(err, errors.New("not running on a pi")) {
		t.Skip("not running on a pi")
		return
	}
	test.That(t, err, test.ShouldBeNil)

	p := pp.(*piPigpio)

	defer func() {
		err := p.Close(ctx)
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
		err = p.SetGPIOBcom(26, false)
		test.That(t, err, test.ShouldBeNil)

		v, err := reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)

		// try to set high
		err = p.SetGPIOBcom(26, true)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 1023, 150)

		// back to low
		err = p.SetGPIOBcom(26, false)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.SetGPIOBcom(18, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		i1, ok := p.DigitalInterruptByName("i1")
		test.That(t, ok, test.ShouldBeTrue)
		before, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(18, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	t.Run("servo in/out", func(t *testing.T) {
		servoReg := registry.ComponentLookup(servo.Subtype, picommon.ModelName)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(ctx, nil, config.Component{Name: "servo", Attributes: config.AttributeMap{"pin": "18"}}, logger)
		test.That(t, err, test.ShouldBeNil)
		servo1 := servoInt.(servo.Servo)

		err = servo1.Move(ctx, 90)
		test.That(t, err, test.ShouldBeNil)

		v, err := servo1.GetPosition(ctx)
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
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return pp, nil
	}

	motorReg := registry.ComponentLookup(motor.Subtype, picommon.ModelName)
	test.That(t, motorReg, test.ShouldNotBeNil)

	motorInt, err := motorReg.Constructor(ctx, &injectRobot, config.Component{
		Name: "motor1", ConvertedAttributes: &motor.Config{
			Pins: motor.PinConfig{
				A:   "13", // bcom 27
				B:   "40", // bcom 21
				PWM: "7",  // bcom 4
			},
			EncoderA:         "hall-a",
			EncoderB:         "hall-b",
			TicksPerRotation: 200,
			BoardName:        "test",
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	motor1 := motorInt.(motor.Motor)

	t.Run("motor forward", func(t *testing.T) {
		pos, err := motor1.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldAlmostEqual, .0, 0o1)

		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err = motor1.GoFor(ctx, 15, 2)
		test.That(t, err, test.ShouldBeNil)
		on, err := motor1.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		hallA, ok := p.DigitalInterruptByName("hall-a")
		test.That(t, ok, test.ShouldBeTrue)
		hallB, ok := p.DigitalInterruptByName("hall-b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := motor1.IsPowered(ctx)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)

			loops++
			if loops > 100 {
				pos, err = motor1.GetPosition(ctx)
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
		err := motor1.GoFor(ctx, -15, 2)
		test.That(t, err, test.ShouldBeNil)

		on, err := motor1.IsPowered(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)

		hallA, ok := p.DigitalInterruptByName("hall-a")
		test.That(t, ok, test.ShouldBeTrue)
		hallB, ok := p.DigitalInterruptByName("hall-b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, err := motor1.IsPowered(ctx)
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
	ResourceByNameFunc func(name resource.Name) (interface{}, error)
}

// ResourceByName calls the injected ResourceByName or the real version.
func (r *Robot) ResourceByName(name resource.Name) (interface{}, error) {
	if r.ResourceByNameFunc == nil {
		return r.Robot.ResourceByName(name)
	}
	return r.ResourceByNameFunc(name)
}
