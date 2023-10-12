//go:build linux && (arm64 || arm)

package piimpl

import (
	"context"
	"os"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/encoder/incremental"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/gpio"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestPiHardware(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external pi hardware tests")
		return
	}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	cfg := genericlinux.Config{
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"},                     // plug physical 12(18) into this (17)
			{Name: "servo-i", Pin: "22", Type: "servo"}, // bcom-25
			{Name: "a", Pin: "33"},                      // bcom 13
			{Name: "b", Pin: "37"},                      // bcom 26
		},
	}
	resourceConfig := resource.Config{ConvertedAttributes: &cfg}

	pp, err := newPigpio(ctx, board.Named("foo"), resourceConfig, logger)
	if os.Getuid() != 0 || err != nil && err.Error() == "not running on a pi" {
		t.Skip("not running as root on a pi")
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

		v, err := reader.Read(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)

		// try to set high
		err = p.SetGPIOBcom(26, true)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 1023, 150)

		// back to low
		err = p.SetGPIOBcom(26, false)
		test.That(t, err, test.ShouldBeNil)

		v, err = reader.Read(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldAlmostEqual, 0, 150)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.SetGPIOBcom(18, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		i1, ok := p.DigitalInterruptByName("i1")
		test.That(t, ok, test.ShouldBeTrue)
		before, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(18, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	t.Run("servo in/out", func(t *testing.T) {
		servoReg, ok := resource.LookupRegistration(servo.API, picommon.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &picommon.ServoConfig{Pin: "18"},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		servo1 := servoInt.(servo.Servo)

		err = servo1.Move(ctx, 90, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, int(v), test.ShouldEqual, 90)

		time.Sleep(300 * time.Millisecond)

		servoI, ok := p.DigitalInterruptByName("servo-i")
		test.That(t, ok, test.ShouldBeTrue)
		val, err := servoI.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, val, test.ShouldAlmostEqual, int64(1500), 500) // this is a tad noisy
	})

	motorReg, ok := resource.LookupRegistration(motor.API, picommon.Model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	encoderReg, ok := resource.LookupRegistration(encoder.API, resource.DefaultModelFamily.WithModel("encoder"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, encoderReg, test.ShouldNotBeNil)

	deps := make(resource.Dependencies)
	_, err = encoderReg.Constructor(ctx, deps, resource.Config{
		Name: "encoder1", ConvertedAttributes: &incremental.Config{
			Pins: incremental.Pins{
				A: "a",
				B: "b",
			},
			BoardName: "test",
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	motorDeps := make([]string, 0)
	motorDeps = append(motorDeps, "encoder1")

	motorInt, err := motorReg.Constructor(ctx, deps, resource.Config{
		Name: "motor1", ConvertedAttributes: &gpio.Config{
			Pins: gpio.PinConfig{
				A:   "13", // bcom 27
				B:   "40", // bcom 21
				PWM: "7",  // bcom 4
			},
			BoardName:        "test",
			Encoder:          "encoder1",
			TicksPerRotation: 200,
		},
		DependsOn: motorDeps,
	}, logger)
	test.That(t, err, test.ShouldBeNil)
	motor1 := motorInt.(motor.Motor)

	t.Run("motor forward", func(t *testing.T) {
		pos, err := motor1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldAlmostEqual, .0, 0o1)

		// 15 rpm is about what we can get from 5v. 2 rotations should take 8 seconds
		err = motor1.GoFor(ctx, 15, 2, nil)
		test.That(t, err, test.ShouldBeNil)
		on, powerPct, err := motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		encA, ok := p.DigitalInterruptByName("a")
		test.That(t, ok, test.ShouldBeTrue)
		encB, ok := p.DigitalInterruptByName("b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, _, err := motor1.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)

			loops++
			if loops > 100 {
				pos, err = motor1.Position(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				aVal, err := encA.Value(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
				bVal, err := encB.Value(context.Background(), nil)
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
		err := motor1.GoFor(ctx, -15, 2, nil)
		test.That(t, err, test.ShouldBeNil)

		on, powerPct, err := motor1.IsPowered(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, on, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 1.0)

		encA, ok := p.DigitalInterruptByName("a")
		test.That(t, ok, test.ShouldBeTrue)
		encB, ok := p.DigitalInterruptByName("b")
		test.That(t, ok, test.ShouldBeTrue)

		loops := 0
		for {
			on, _, err := motor1.IsPowered(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if !on {
				break
			}

			time.Sleep(100 * time.Millisecond)
			loops++
			aVal, err := encA.Value(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)
			bVal, err := encB.Value(context.Background(), nil)
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
