//go:build linux && (arm64 || arm) && !no_pigpio

package piimpl

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

func TestPiPigpio(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg := genericlinux.Config{
		DigitalInterrupts: []board.DigitalInterruptConfig{
			{Name: "i1", Pin: "11"}, // bcom 17
			{Name: "servo-i", Pin: "22", Type: "servo"},
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

	t.Run("gpio and pwm", func(t *testing.T) {
		pin, err := p.GPIOPinByName("29")
		test.That(t, err, test.ShouldBeNil)

		// try to set high
		err = pin.Set(ctx, true, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err := pin.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, true)

		// try to set low
		err = pin.Set(ctx, false, nil)
		test.That(t, err, test.ShouldBeNil)

		v, err = pin.Get(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, v, test.ShouldEqual, false)

		// pwm 50%
		err = pin.SetPWM(ctx, 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		vF, err := pin.PWM(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.5, 0.01)

		// 4000 hz
		err = pin.SetPWMFreq(ctx, 4000, nil)
		test.That(t, err, test.ShouldBeNil)

		vI, err := pin.PWMFreq(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 4000)

		// 90%
		err = pin.SetPWM(ctx, 0.9, nil)
		test.That(t, err, test.ShouldBeNil)

		vF, err = pin.PWM(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vF, test.ShouldAlmostEqual, 0.9, 0.01)

		// 8000hz
		err = pin.SetPWMFreq(ctx, 8000, nil)
		test.That(t, err, test.ShouldBeNil)

		vI, err = pin.PWMFreq(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vI, test.ShouldEqual, 8000)
	})

	t.Run("basic interrupts", func(t *testing.T) {
		err = p.SetGPIOBcom(17, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		i1, ok := p.DigitalInterruptByName("i1")
		test.That(t, ok, test.ShouldBeTrue)
		before, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(17, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err := i1.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))

		err = p.SetGPIOBcom(27, false)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)
		_, ok = p.DigitalInterruptByName("some")
		test.That(t, ok, test.ShouldBeFalse)
		i2, ok := p.DigitalInterruptByName("13")
		test.That(t, ok, test.ShouldBeTrue)
		before, err = i2.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		err = p.SetGPIOBcom(27, true)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(5 * time.Millisecond)

		after, err = i2.Value(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, after-before, test.ShouldEqual, int64(1))

		_, ok = p.DigitalInterruptByName("11")
		test.That(t, ok, test.ShouldBeTrue)
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
				ConvertedAttributes: &picommon.ServoConfig{Pin: "22"},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		servo1 := servoInt.(servo.Servo)

		err = servo1.Move(ctx, 90, nil)
		test.That(t, err, test.ShouldBeNil)

		err = servo1.Move(ctx, 190, nil)
		test.That(t, err, test.ShouldNotBeNil)

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

	t.Run("servo initialize with pin error", func(t *testing.T) {
		servoReg, ok := resource.LookupRegistration(servo.API, picommon.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		_, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &picommon.ServoConfig{Pin: ""},
			},
			logger,
		)
		test.That(t, err.Error(), test.ShouldContainSubstring, "need pin for pi servo")
	})

	t.Run("check new servo defaults", func(t *testing.T) {
		ctx := context.Background()
		servoReg, ok := resource.LookupRegistration(servo.API, picommon.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &picommon.ServoConfig{Pin: "22"},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		servo1 := servoInt.(servo.Servo)
		pos1, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos1, test.ShouldEqual, 90)
	})

	t.Run("check set default position", func(t *testing.T) {
		ctx := context.Background()
		servoReg, ok := resource.LookupRegistration(servo.API, picommon.Model)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, servoReg, test.ShouldNotBeNil)

		initPos := 33.0
		servoInt, err := servoReg.Constructor(
			ctx,
			nil,
			resource.Config{
				Name:                "servo",
				ConvertedAttributes: &picommon.ServoConfig{Pin: "22", StartPos: &initPos},
			},
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		servo1 := servoInt.(servo.Servo)
		pos1, err := servo1.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos1, test.ShouldEqual, 33)

		localServo := servo1.(*piPigpioServo)
		test.That(t, localServo.holdPos, test.ShouldBeTrue)
	})
}

func TestServoFunctions(t *testing.T) {
	t.Run("check servo math", func(t *testing.T) {
		pw := angleToPulseWidth(1, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 511)
		pw = angleToPulseWidth(0, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 500)
		pw = angleToPulseWidth(179, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 2488)
		pw = angleToPulseWidth(180, servoDefaultMaxRotation)
		test.That(t, pw, test.ShouldEqual, 2500)
		pw = angleToPulseWidth(179, 270)
		test.That(t, pw, test.ShouldEqual, 1825)
		pw = angleToPulseWidth(180, 270)
		test.That(t, pw, test.ShouldEqual, 1833)
		a := pulseWidthToAngle(511, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 1)
		a = pulseWidthToAngle(500, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 0)
		a = pulseWidthToAngle(2500, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 180)
		a = pulseWidthToAngle(2488, servoDefaultMaxRotation)
		test.That(t, a, test.ShouldEqual, 179)
		a = pulseWidthToAngle(1825, 270)
		test.That(t, a, test.ShouldEqual, 179)
		a = pulseWidthToAngle(1833, 270)
		test.That(t, a, test.ShouldEqual, 180)
	})

	t.Run(("check Move IsMoving ande pigpio errors"), func(t *testing.T) {
		ctx := context.Background()
		s := &piPigpioServo{pinname: "1", maxRotation: 180, opMgr: operation.NewSingleOperationManager()}

		s.res = -93
		err := s.pigpioErrors(int(s.res))
		test.That(t, err.Error(), test.ShouldContainSubstring, "pulsewidths")
		moving, err := s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		s.res = -7
		err = s.pigpioErrors(int(s.res))
		test.That(t, err.Error(), test.ShouldContainSubstring, "range")
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)

		s.res = 0
		err = s.pigpioErrors(int(s.res))
		test.That(t, err, test.ShouldBeNil)
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		s.res = 1
		err = s.pigpioErrors(int(s.res))
		test.That(t, err, test.ShouldBeNil)
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		err = s.pigpioErrors(-4)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed")
		moving, err = s.IsMoving(ctx)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, err, test.ShouldBeNil)

		err = s.Move(ctx, 8, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = s.Stop(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, err := s.Position(ctx, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0)
	})
}
