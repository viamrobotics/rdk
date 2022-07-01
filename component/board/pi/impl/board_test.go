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
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := board.Config{
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"}, // bcom 17
			{Name: "servo-i", Pin: "22", Type: "servo"},
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

	t.Run("gpio and pwm", func(t *testing.T) {
		pin, err := p.GPIOPinByName("29")
		test.That(t, err, test.ShouldBeNil)

		// try to set high
		err = pin.Set(ctx, true)
		test.That(t, err, test.ShouldBeNil)

		v, err := pin.Get(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		// try to set low
		err = pin.Set(ctx, false)
		test.That(t, err, test.ShouldBeNil)

		v, err = pin.Get(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, false)

		// pwm 50%
		err = pin.SetPWM(ctx, 0.5)
		test.That(t, err, test.ShouldBeNil)

		vF, err := pin.PWM(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.5, 0.01)

		// 4000 hz
		err = pin.SetPWMFreq(ctx, 4000)
		test.That(t, err, test.ShouldBeNil)

		vI, err := pin.PWMFreq(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 4000)

		// 90%
		err = pin.SetPWM(ctx, 0.9)
		test.That(t, err, test.ShouldBeNil)

		vF, err = pin.PWM(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.9, 0.01)

		// 8000hz
		err = pin.SetPWMFreq(ctx, 8000)
		test.That(t, err, test.ShouldBeNil)

		vI, err = pin.PWMFreq(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 8000)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.SetGPIOBcom(17, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		i1, ok := p.DigitalInterruptByName("i1")
		test.That(t, ok, test.ShouldBeTrue)
		before, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(17, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))
	})

	//nolint:dupl
	t.Run("servo in/out", func(t *testing.T) {
		servoReg := registry.ComponentLookup(servo.Subtype, picommon.ModelName)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			config.Component{
				Name:                "servo",
				ConvertedAttributes: &picommon.ServoConfig{Pin: "22"},
			},
			logger,
		)
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
}

func TestNewServo(t *testing.T) {
	fakecfg := board.Config{
		
	}

}
